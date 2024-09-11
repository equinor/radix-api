package prometheus

import (
	"context"
	"fmt"
	"time"

	applicationModels "github.com/equinor/radix-api/api/applications/models"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	prometheusApi "github.com/prometheus/client_golang/api"
	prometheusV1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/api/resource"
)

type queryName string

const (
	cpuMax    queryName = "cpuMax"
	cpuMin    queryName = "cpuMin"
	cpuAvg    queryName = "cpuAvg"
	memoryMax queryName = "memoryMax"
	memoryMin queryName = "memoryMin"
	memoryAvg queryName = "memoryAvg"
)

// GetUsedResources Get used resources for the application
func GetUsedResources(ctx context.Context, appName, period, prometheusUrl string, _, _ []string) (*applicationModels.UsedResources, error) {
	log.Ctx(ctx).Debug().Msgf("Getting used resources for application %s", appName)
	client, err := prometheusApi.NewClient(prometheusApi.Config{Address: prometheusUrl})
	if err != nil {
		return nil, fmt.Errorf("failed to create the Prometheus client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute) // replace with 10*time.Seconds
	defer cancel()

	api := prometheusV1.NewAPI(client)
	results := make(map[queryName]model.Value)
	now := time.Now()
	for metricName, query := range getPrometheusQueries(appName, period) {
		result, warnings, err := api.Query(ctx, query, now)
		if err != nil {
			return nil, fmt.Errorf("failed to get Prometheus metrics: %w", err)
		}
		if len(warnings) > 0 {
			log.Ctx(ctx).Warn().Msgf("Warnings: %v\n", warnings)
		}
		results[metricName] = result
	}
	resources := applicationModels.UsedResources{
		From: radixutils.FormatTimestamp(now.Add(-time.Hour * 24 * 30)),
		To:   radixutils.FormatTimestamp(now),
		CPU: &applicationModels.UsedResource{
			Min:     getCpuMetricValue(ctx, results, cpuMin),
			Max:     getCpuMetricValue(ctx, results, cpuMax),
			Average: getCpuMetricValue(ctx, results, cpuAvg),
		},
		Memory: &applicationModels.UsedResource{
			Min:     getMemoryMetricValue(ctx, results, memoryMin),
			Max:     getMemoryMetricValue(ctx, results, memoryMax),
			Average: getMemoryMetricValue(ctx, results, memoryAvg),
		},
	}
	log.Ctx(ctx).Debug().Msgf("Got used resources for application %s", appName)
	return &resources, nil
}

func getCpuMetricValue(ctx context.Context, queryResults map[queryName]model.Value, queryName queryName) string {
	if value, ok := getSummedMetricsValue(ctx, queryResults, queryName); ok {
		quantity := resource.NewMilliQuantity(int64(value*1000.0), resource.BinarySI)
		return quantity.String()
	}
	return ""
}

func getMemoryMetricValue(ctx context.Context, queryResults map[queryName]model.Value, queryName queryName) string {
	if value, ok := getSummedMetricsValue(ctx, queryResults, queryName); ok {
		quantity := resource.NewScaledQuantity(int64(value/1000.0), resource.Mega)
		return quantity.String()
	}
	return ""
}

func getSummedMetricsValue(ctx context.Context, queryResults map[queryName]model.Value, queryName queryName) (float64, bool) {
	queryResult, ok := queryResults[queryName]
	if !ok {
		return 0, false
	}
	groupedMetrics, ok := queryResult.(model.Vector)
	if !ok {
		log.Ctx(ctx).Error().Msgf("Failed to convert metrics query %s result to Vector", queryName)
		return 0, false
	}
	values := slice.Reduce(groupedMetrics, make([]float64, 0), func(acc []float64, sample *model.Sample) []float64 {
		return append(acc, float64(sample.Value))
	})
	if len(values) == 0 {
		return 0, false
	}
	switch queryName {
	case cpuMax, memoryMax:
		max := slice.Reduce(values, values[0], func(maxValue, sample float64) float64 {
			if maxValue < sample {
				return sample
			}
			return maxValue
		})
		return max, true
	case cpuMin, memoryMin:
		min := slice.Reduce(values, values[0], func(minValue, sample float64) float64 {
			if minValue > sample {
				return sample
			}
			return minValue
		})
		return min, true
	case cpuAvg, memoryAvg:
		avg := slice.Reduce(values, 0, func(sum, sample float64) float64 {
			return sum + sample
		}) / float64(len(values))
		return avg, true
	}
	return 0, false
}

func getPrometheusQueries(appName string, period string) map[queryName]string {
	cpuUsageQuery := fmt.Sprintf(`(sum(rate(container_cpu_usage_seconds_total{namespace=~"%[1]s-.*",namespace!="%[1]s-app",}[5m])) by (namespace,container)[%s:])`, appName, period)
	memoryUsageQuery := fmt.Sprintf(`(sum(rate(container_memory_usage_bytes{namespace=~"%[1]s-.*",namespace!="%[1]s-app",}[5m])) by (namespace,container)[%s:])`, appName, period)

	return map[queryName]string{
		cpuMax:    fmt.Sprintf("max_over_time%s", cpuUsageQuery),
		cpuMin:    fmt.Sprintf("min_over_time%s", cpuUsageQuery),
		cpuAvg:    fmt.Sprintf("avg_over_time%s", cpuUsageQuery),
		memoryMax: fmt.Sprintf("max_over_time%s", memoryUsageQuery),
		memoryMin: fmt.Sprintf("min_over_time%s", memoryUsageQuery),
		memoryAvg: fmt.Sprintf("avg_over_time%s", memoryUsageQuery),
	}
}
