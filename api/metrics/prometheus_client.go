package metrics

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/equinor/radix-api/api/metrics/internal"
	"github.com/equinor/radix-api/api/utils/logs"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	prometheusApi "github.com/prometheus/client_golang/api"
	prometheusV1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	prometheusModel "github.com/prometheus/common/model"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// PrometheusClient Interface for Prometheus client
type PrometheusClient interface {
	// GetMetrics Get metrics for the application
	GetMetrics(ctx context.Context, appName, envName, componentName, duration, since string) (map[internal.QueryName]prometheusModel.Value, []string, error)
}

// NewPrometheusClient Constructor for Prometheus client
func NewPrometheusClient(prometheusUrl string) (PrometheusClient, error) {
	roundTripLogger := logs.Logger(func(e *zerolog.Event) {
		e.Str("client", "prometheus")
	})
	apiClient, err := prometheusApi.NewClient(prometheusApi.Config{Address: prometheusUrl, RoundTripper: roundTripLogger(prometheusApi.DefaultRoundTripper)})
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
			log.Ctx(ctx).Error().Err(err).Msgf("Failed to get Prometheus metrics for the query: %s", query)
			return nil, nil, errors.New("failed to get Prometheus metrics")
		}
		if len(resultWarnings) > 0 {
			log.Ctx(ctx).Warn().Strs("warnings", resultWarnings).Msg("promethus warnings")
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
	componentFilter := radixutils.TernaryString(componentName == "", "", fmt.Sprintf(`,container="%s"`, componentName))
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

type MetricsByPodResponse struct {
	ResoureceRequests struct {
		Memory map[string]float64
		Cpu    map[string]float64
	}
	MaxUsage map[string]float64
}

func (c *client) GetMetricsByPod(ctx context.Context, appName, envName string, duration time.Duration) (MetricsByPodResponse, error) {
	respone := MetricsByPodResponse{}
	namespace := ".*"
	if envName != "" {
		namespace = envName
	}

	resurceRequets := fmt.Sprintf(`max by(container, pod, resource) (kube_pod_container_resource_requests{namespace!="%s-app", namespace=~"%s-%s"})`, appName, appName, namespace)
	cpuUsage := fmt.Sprintf(`max by (container, pod) (rate(container_cpu_usage_seconds_total{namespace!="%s-app", namespace="%s-%s"} [%s]))`, appName, appName, namespace, duration)
	memoryUsage := fmt.Sprintf(`max_over_time(max by(container, pod) (container_memory_usage_bytes{namespace!="%s-app", namespace="%s-%s"})[%s:])`, appName, appName, namespace, duration)
	value, w, err := c.api.Query(ctx, resurceRequets, time.Now())
	if err != nil {
		return MetricsByPodResponse{}, err
	}
	if len(w) > 0 {
		log.Ctx(ctx).Warn().Strs("warnings", w).Msgf("warnings fetching resource requests")
	}

	value, w, err = c.api.Query(ctx, cpuUsage, time.Now())
	if err != nil {
		return MetricsByPodResponse{}, err
	}
	if len(w) > 0 {
		log.Ctx(ctx).Warn().Strs("warnings", w).Msgf("warnings fetching cpu usage")
	}
	
	value, w, err = c.api.Query(ctx, memoryUsage, time.Now())
	if err != nil {
		return MetricsByPodResponse{}, err
	}
	if len(w) > 0 {
		log.Ctx(ctx).Warn().Strs("warnings", w).Msgf("warnings fetching cpu usage")
	}

	// TODO: Map results to resposne

	return respone, nil
}
