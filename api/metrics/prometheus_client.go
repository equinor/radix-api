package metrics

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/equinor/radix-api/api/metrics/internal"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	prometheusApi "github.com/prometheus/client_golang/api"
	prometheusV1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	prometheusModel "github.com/prometheus/common/model"
	"github.com/rs/zerolog/log"
)

// PrometheusClient Interface for Prometheus client
type PrometheusClient interface {
	// GetMetrics Get metrics for the application
	GetMetrics(ctx context.Context, appName, envName, componentName, duration, since string) (map[internal.QueryName]prometheusModel.Value, []string, error)
}

// NewPrometheusClient Constructor for Prometheus client
func NewPrometheusClient(prometheusUrl string) (PrometheusClient, error) {
	apiClient, err := prometheusApi.NewClient(prometheusApi.Config{Address: prometheusUrl})
	if err != nil {
		return nil, errors.New("failed to create the Prometheus API client")
	}
	api := prometheusV1.NewAPI(apiClient)
	return &client{
		api: api,
	}, nil
}

type client struct {
	api prometheusV1.API
}

// GetMetrics Get metrics for the application
func (c *client) GetMetrics(ctx context.Context, appName, envName, componentName, duration, since string) (map[internal.QueryName]prometheusModel.Value, []string, error) {
	results := make(map[internal.QueryName]model.Value)
	now := time.Now()
	var warnings []string
	for metricName, query := range getPrometheusQueries(appName, envName, componentName, duration, since) {
		result, resultWarnings, err := c.api.Query(ctx, query, now)
		if err != nil {
			log.Ctx(ctx).Error().Msgf("Failed to get Prometheus metrics: %v", err)
			return nil, nil, errors.New("failed to get Prometheus metrics")
		}
		if len(resultWarnings) > 0 {
			log.Ctx(ctx).Warn().Msgf("Warnings: %v\n", resultWarnings)
			warnings = append(warnings, resultWarnings...)
		}
		results[metricName] = result
	}
	return results, warnings, nil
}

func getPrometheusQueries(appName, envName, componentName, duration, since string) map[internal.QueryName]string {
	environmentFilter := radixutils.TernaryString(envName == "",
		fmt.Sprintf(`,namespace=~"%s-.*"`, appName),
		fmt.Sprintf(`,namespace="%s"`, utils.GetEnvironmentNamespace(appName, envName)))
	componentFilter := radixutils.TernaryString(envName == "", "", fmt.Sprintf(`,container="%s"`, componentName))
	offsetFilter := radixutils.TernaryString(since == "", "", fmt.Sprintf(` offset %s `, since))
	cpuUsageQuery := fmt.Sprintf(`sum by (namespace, container) (rate(container_cpu_usage_seconds_total{container!="", namespace!="%s-app" %s %s} [1h])) [%s:] %s`, appName, environmentFilter, componentFilter, duration, offsetFilter)
	memoryUsageQuery := fmt.Sprintf(`sum by (namespace, container) (container_memory_usage_bytes{container!="", namespace!="%s-app" %s %s} > 0) [%s:] %s`, appName, environmentFilter, componentFilter, duration, offsetFilter)
	queries := map[internal.QueryName]string{
		internal.CpuMax:    fmt.Sprintf("max_over_time(%s)", cpuUsageQuery),
		internal.CpuMin:    fmt.Sprintf("min_over_time(%s)", cpuUsageQuery),
		internal.CpuAvg:    fmt.Sprintf("avg_over_time(%s)", cpuUsageQuery),
		internal.MemoryMax: fmt.Sprintf("max_over_time(%s)", memoryUsageQuery),
		internal.MemoryMin: fmt.Sprintf("min_over_time(%s)", memoryUsageQuery),
		internal.MemoryAvg: fmt.Sprintf("avg_over_time(%s)", memoryUsageQuery),
	}
	return queries
}
