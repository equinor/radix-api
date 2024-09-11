package prometheus

import (
	"context"
	"fmt"
	"time"

	applicationModels "github.com/equinor/radix-api/api/applications/models"
	radixutils "github.com/equinor/radix-common/utils"
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

func GetUsedResources(ctx context.Context, appName, period, prometheusUrl string, _, _ []string) (*applicationModels.UsedResources, error) {
	client, err := prometheusApi.NewClient(prometheusApi.Config{Address: prometheusUrl})
	if err != nil {
		return nil, fmt.Errorf("failed to create the Prometheus client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
	return &applicationModels.UsedResources{
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
	}, nil
}

func getCpuMetricValue(ctx context.Context, queryResults map[queryName]model.Value, queryName queryName) string {
	metricsExist, value := getSummedMetricsValue(ctx, queryResults, queryName)
	if metricsExist {
		return resource.NewMilliQuantity(int64(value), resource.BinarySI).String()
	}
	return ""
}

func getMemoryMetricValue(ctx context.Context, queryResults map[queryName]model.Value, queryName queryName) string {
	metricsExist, value := getSummedMetricsValue(ctx, queryResults, queryName)
	if metricsExist {
		return resource.NewQuantity(int64(value), resource.BinarySI).String()
	}
	return ""
}

func getSummedMetricsValue(ctx context.Context, queryResults map[queryName]model.Value, queryName queryName) (bool, float64) {
	queryResult, ok := queryResults[queryName]
	if !ok {
		return false, 0
	}
	groupedMetrics, ok := queryResult.(model.Vector)
	if !ok {
		log.Ctx(ctx).Error().Msgf("Failed to convert metrics query %s result to Vector", queryName)
		return false, 0
	}
	metricsExist := false
	var memoryUsageBytes float64
	for _, sample := range groupedMetrics {
		memoryUsageBytes += float64(sample.Value)
		metricsExist = true
	}
	return metricsExist, memoryUsageBytes
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
