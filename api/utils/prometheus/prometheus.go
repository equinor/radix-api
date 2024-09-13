package prometheus

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"time"

	applicationModels "github.com/equinor/radix-api/api/applications/models"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	prometheusApi "github.com/prometheus/client_golang/api"
	prometheusV1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	prometheusModel "github.com/prometheus/common/model"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/api/resource"
)

type queryName string

const (
	cpuMax             queryName = "cpuMax"
	cpuMin             queryName = "cpuMin"
	cpuAvg             queryName = "cpuAvg"
	memoryMax          queryName = "memoryMax"
	memoryMin          queryName = "memoryMin"
	memoryAvg          queryName = "memoryAvg"
	durationExpression           = `^[0-9]{1,5}[mhdw]$`
	defaultDuration              = "30d"
)

// GetUsedResources Get used resources for the application
func GetUsedResources(ctx context.Context, prometheusUrl, appName, envName, componentName, duration, since string, ignoreZero bool) (*applicationModels.UsedResources, error) {
	durationValue, err := parseQueryDuration(&duration, defaultDuration)
	if err != nil {
		return nil, err
	}
	sinceValue, err := parseQueryDuration(&since, "")
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute) // replace with 10*time.Seconds
	defer cancel()

	log.Ctx(ctx).Debug().Msgf("Getting used resources for application %s", appName)
	results, err := getPrometheusMetrics(ctx, prometheusUrl, appName, envName, componentName, duration, since)
	if err != nil {
		return nil, err
	}
	resources := getUsedResourcesByMetrics(ctx, results, durationValue, sinceValue, ignoreZero)
	log.Ctx(ctx).Debug().Msgf("Got used resources for application %s", appName)
	return resources, nil
}

func getUsedResourcesByMetrics(ctx context.Context, results map[queryName]prometheusModel.Value, queryDuration time.Duration, querySince time.Duration, ignoreZero bool) *applicationModels.UsedResources {
	usedCpuResource := applicationModels.UsedResource{}
	usedCpuResource.Min, usedCpuResource.MinActual = getCpuMetricValue(ctx, results, cpuMin, ignoreZero)
	usedCpuResource.Max, usedCpuResource.MaxActual = getCpuMetricValue(ctx, results, cpuMax, ignoreZero)
	usedCpuResource.Average, usedCpuResource.AvgActual = getCpuMetricValue(ctx, results, cpuAvg, ignoreZero)
	usedMemoryResource := applicationModels.UsedResource{}
	usedMemoryResource.Min, usedMemoryResource.MinActual = getMemoryMetricValue(ctx, results, memoryMin, ignoreZero)
	usedMemoryResource.Max, usedMemoryResource.MaxActual = getMemoryMetricValue(ctx, results, memoryMax, ignoreZero)
	usedMemoryResource.Average, usedMemoryResource.AvgActual = getMemoryMetricValue(ctx, results, memoryAvg, ignoreZero)
	now := time.Now()
	return &applicationModels.UsedResources{
		From:   radixutils.FormatTimestamp(now.Add(-queryDuration)),
		To:     radixutils.FormatTimestamp(now.Add(-querySince)),
		CPU:    &usedCpuResource,
		Memory: &usedMemoryResource,
	}
}

func getPrometheusMetrics(ctx context.Context, prometheusUrl, appName, envName, componentName, duration, since string) (map[queryName]prometheusModel.Value, error) {
	client, err := prometheusApi.NewClient(prometheusApi.Config{Address: prometheusUrl})
	if err != nil {
		return nil, fmt.Errorf("failed to create the Prometheus client: %w", err)
	}
	api := prometheusV1.NewAPI(client)
	results := make(map[queryName]model.Value)
	now := time.Now()
	for metricName, query := range getPrometheusQueries(appName, envName, componentName, duration, since) {
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

func parseQueryDuration(duration *string, defaultValue string) (time.Duration, error) {
	if duration == nil || len(*duration) == 0 || !regexp.MustCompile(durationExpression).MatchString(*duration) {
		duration = pointers.Ptr(defaultValue)
	}
	if duration == nil || len(*duration) == 0 {
		return 0, nil
	}
	parsedDuration, err := prometheusModel.ParseDuration(*duration)
	return time.Duration(parsedDuration), err
}

func roundActualValue(num float64) float64 {
	return math.Round(num*1e6) / 1e6
}

func getCpuMetricValue(ctx context.Context, queryResults map[queryName]prometheusModel.Value, queryName queryName, ignoreZero bool) (string, *float64) {
	if value, ok := getMetricsValue(ctx, queryResults, queryName, ignoreZero); ok {
		valueInMillicores := value * 1000.0
		quantity := resource.NewMilliQuantity(int64(valueInMillicores), resource.BinarySI)
		return quantity.String(), pointers.Ptr(roundActualValue(valueInMillicores))
	}
	return "", nil
}

func getMemoryMetricValue(ctx context.Context, queryResults map[queryName]prometheusModel.Value, queryName queryName, ignoreZero bool) (string, *float64) {
	if value, ok := getMetricsValue(ctx, queryResults, queryName, ignoreZero); ok {
		valueInMegabytes := value / 1000.0
		quantity := resource.NewScaledQuantity(int64(valueInMegabytes), resource.Mega)
		return quantity.String(), pointers.Ptr(roundActualValue(valueInMegabytes))
	}
	return "", nil
}

func getMetricsValue(ctx context.Context, queryResults map[queryName]prometheusModel.Value, queryName queryName, ignoreZero bool) (float64, bool) {
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
		if ignoreZero && sample.Value <= 0 {
			return acc
		}
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

func getPrometheusQueries(appName, envName, componentName, duration, since string) map[queryName]string {
	environmentFilter := radixutils.TernaryString(envName == "",
		fmt.Sprintf(`,namespace=~"%s-.*"`, appName),
		fmt.Sprintf(`,namespace="%s"`, utils.GetEnvironmentNamespace(appName, envName)))
	componentFilter := radixutils.TernaryString(envName == "", "", fmt.Sprintf(`,container="%s"`, componentName))
	offsetFilter := radixutils.TernaryString(since == "", "", fmt.Sprintf(` offset %s `, since))
	cpuUsageQuery := fmt.Sprintf(`sum(rate(container_cpu_usage_seconds_total{namespace!="%s-app" %s %s}[5m] %s )) by (namespace,container)[%s:]`, appName, environmentFilter, componentFilter, offsetFilter, duration)
	memoryUsageQuery := fmt.Sprintf(`sum(rate(container_memory_usage_bytes{namespace!="%s-app" %s %s}[5m] %s )) by (namespace,container)[%s:]`, appName, environmentFilter, componentFilter, offsetFilter, duration)
	queries := map[queryName]string{
		cpuMax:    fmt.Sprintf("max_over_time(%s)", cpuUsageQuery),
		cpuMin:    fmt.Sprintf("min_over_time(%s)", cpuUsageQuery),
		cpuAvg:    fmt.Sprintf("avg_over_time(%s)", cpuUsageQuery),
		memoryMax: fmt.Sprintf("max_over_time(%s)", memoryUsageQuery),
		memoryMin: fmt.Sprintf("min_over_time(%s)", memoryUsageQuery),
		memoryAvg: fmt.Sprintf("avg_over_time(%s)", memoryUsageQuery),
	}
	return queries
}
