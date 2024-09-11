package prometheus

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	applicationModels "github.com/equinor/radix-api/api/applications/models"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	prometheusApi "github.com/prometheus/client_golang/api"
	prometheusV1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/api/resource"
)

type queryName string

const (
	cpuMax           queryName = "cpuMax"
	cpuMin           queryName = "cpuMin"
	cpuAvg           queryName = "cpuAvg"
	memoryMax        queryName = "memoryMax"
	memoryMin        queryName = "memoryMin"
	memoryAvg        queryName = "memoryAvg"
	periodExpression           = `^[0-9]{1,5}[mhdw]$`
)

// GetUsedResources Get used resources for the application
func GetUsedResources(ctx context.Context, prometheusUrl, appName, envName, componentName, period string) (*applicationModels.UsedResources, error) {
	err := validatePeriod(period)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute) // replace with 10*time.Seconds
	defer cancel()

	log.Ctx(ctx).Debug().Msgf("Getting used resources for application %s", appName)
	results, err := getPrometheusMetrics(ctx, prometheusUrl, appName, envName, componentName, period)
	if err != nil {
		return nil, err
	}
	resources := getUsedResourcesByMetrics(ctx, results)
	log.Ctx(ctx).Debug().Msgf("Got used resources for application %s", appName)
	return resources, nil
}

func getUsedResourcesByMetrics(ctx context.Context, results map[queryName]model.Value) *applicationModels.UsedResources {
	now := time.Now()
	resources := applicationModels.UsedResources{
		From: radixutils.FormatTimestamp(now.Add(-time.Hour * 24 * 30)), // TODO change this corresponding the requested period
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
	return &resources
}

func getPrometheusMetrics(ctx context.Context, prometheusUrl, appName, envName, componentName, period string) (map[queryName]model.Value, error) {
	client, err := prometheusApi.NewClient(prometheusApi.Config{Address: prometheusUrl})
	if err != nil {
		return nil, fmt.Errorf("failed to create the Prometheus client: %w", err)
	}
	api := prometheusV1.NewAPI(client)
	results := make(map[queryName]model.Value)
	now := time.Now()
	for metricName, query := range getPrometheusQueries(appName, envName, componentName, period) {
		result, warnings, err := api.Query(ctx, query, now)
		if err != nil {
			return nil, fmt.Errorf("failed to get Prometheus metrics: %w", err)
		}
		if len(warnings) > 0 {
			log.Ctx(ctx).Warn().Msgf("Warnings: %v\n", warnings)
		}
		results[metricName] = result
	}
	return results, nil
}

func validatePeriod(period string) error {
	if len(period) > 0 && !regexp.MustCompile(periodExpression).MatchString(period) {
		return errors.New("invalid period format")
	}
	return nil
}

func getCpuMetricValue(ctx context.Context, queryResults map[queryName]model.Value, queryName queryName) string {
	if value, ok := getMetricsValue(ctx, queryResults, queryName); ok {
		quantity := resource.NewMilliQuantity(int64(value*1000.0), resource.BinarySI)
		return quantity.String()
	}
	return ""
}

func getMemoryMetricValue(ctx context.Context, queryResults map[queryName]model.Value, queryName queryName) string {
	if value, ok := getMetricsValue(ctx, queryResults, queryName); ok {
		quantity := resource.NewScaledQuantity(int64(value/1000.0), resource.Mega)
		return quantity.String()
	}
	return ""
}

func getMetricsValue(ctx context.Context, queryResults map[queryName]model.Value, queryName queryName) (float64, bool) {
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
		maxVal := slice.Reduce(values, values[0], func(maxValue, sample float64) float64 {
			if maxValue < sample {
				return sample
			}
			return maxValue
		})
		return maxVal, true
	case cpuMin, memoryMin:
		minVal := slice.Reduce(values, values[0], func(minValue, sample float64) float64 {
			if minValue > sample {
				return sample
			}
			return minValue
		})
		return minVal, true
	case cpuAvg, memoryAvg:
		avgVal := slice.Reduce(values, 0, func(sum, sample float64) float64 {
			return sum + sample
		}) / float64(len(values))
		return avgVal, true
	}
	return 0, false
}

func getPrometheusQueries(appName, envName, componentName, period string) map[queryName]string {
	if period == "" {
		period = "30d"
	}
	environmentFilter := radixutils.TernaryString(envName == "",
		fmt.Sprintf(`,namespace=~"%s-.*"`, appName),
		fmt.Sprintf(`,namespace="%s"`, utils.GetEnvironmentNamespace(appName, envName)))
	componentFilter := radixutils.TernaryString(envName == "", "", fmt.Sprintf(`,container="%s"`, componentName))
	cpuUsageRateQuery := fmt.Sprintf(`rate(container_cpu_usage_seconds_total{namespace!="%s-app"%s%s}[5m])) by (namespace,container)[%s:]`, appName, environmentFilter, componentFilter, period)
	memoryUsageRateQuery := fmt.Sprintf(`rate(container_memory_usage_bytes{namespace!="%s-app"%s%s}[5m])) by (namespace,container)[%s:]`, appName, environmentFilter, componentFilter, period)
	return map[queryName]string{
		cpuMax:    fmt.Sprintf("max_over_time(sum(%s)", cpuUsageRateQuery),
		cpuMin:    fmt.Sprintf("min_over_time(sum(%s)", cpuUsageRateQuery),
		cpuAvg:    fmt.Sprintf("avg_over_time(sum(%s)", cpuUsageRateQuery),
		memoryMax: fmt.Sprintf("max_over_time(sum(%s)", memoryUsageRateQuery),
		memoryMin: fmt.Sprintf("min_over_time(sum(%s)", memoryUsageRateQuery),
		memoryAvg: fmt.Sprintf("avg_over_time(sum(%s)", memoryUsageRateQuery),
	}
}
