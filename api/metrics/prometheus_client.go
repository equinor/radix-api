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
	GetMetricsByPod(ctx context.Context, appName, envName, duration string) (map[internal.QueryName][]QueryVectorResult, error)
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

type ResourceCategory struct {
	Memory prometheusModel.SampleValue
	Cpu    prometheusModel.SampleValue
}

type Containers struct {
	Requests ResourceCategory
	Replicas map[string]ResourceCategory
}

func (c *client) GetMetricsByPod(ctx context.Context, appName, envName, duration string) (map[internal.QueryName][]QueryVectorResult, error) {
	namespace := ".*"
	if envName != "" {
		namespace = envName
	}

	queries := map[internal.QueryName]string{
		internal.CpuRequests:   fmt.Sprintf(`max by(namespace, container, pod) (kube_pod_container_resource_requests{container!="",namespace!="%s-app", namespace=~"%s-%s",resource="cpu"}) * on(pod) group_left(label_radix_component) kube_pod_labels{label_radix_component!=""}`, appName, appName, namespace),
		internal.MemoryRequest: fmt.Sprintf(`max by(namespace, container, pod) (kube_pod_container_resource_requests{container!="",namespace!="%s-app", namespace=~"%s-%s",resource="memory"}) * on(pod) group_left(label_radix_component) kube_pod_labels{label_radix_component!=""}`, appName, appName, namespace),
		internal.CpuMax:        fmt.Sprintf(`max by(namespace, container, pod) (max_over_time(rate(container_cpu_usage_seconds_total{container!="",namespace!="%s-app", namespace=~"%s-%s"}[1m]) [%s:1m])) * on(pod) group_left(label_radix_component) kube_pod_labels{label_radix_component!=""}`, appName, appName, namespace, duration),
		internal.MemoryMax:     fmt.Sprintf(`max by(namespace, container, pod) (max_over_time(container_memory_usage_bytes{container!="",namespace!="%s-app", namespace=~"%s-%s"} [%s:1m])) * on(pod) group_left(label_radix_component) kube_pod_labels{label_radix_component!=""}`, appName, appName, namespace, duration),
	}

	results := make(map[internal.QueryName][]QueryVectorResult)
	for queryName, query := range queries {
		values, err := c.queryVector(ctx, query)
		if err != nil {
			return nil, err
		}
		results[queryName] = values
	}

	return results, nil
}

type QueryVectorResult struct {
	Labels map[string]string
	Value  float64
}

func (c *client) queryVector(ctx context.Context, query string) ([]QueryVectorResult, error) {
	response, w, err := c.api.Query(ctx, query, time.Now())
	if err != nil {
		return nil, err
	}
	if len(w) > 0 {
		log.Ctx(ctx).Warn().Str("query", query).Strs("warnings", w).Msgf("fetching vector query")
	} else {
		log.Ctx(ctx).Trace().Str("query", query).Msgf("fetching vector query")
	}

	r, ok := response.(prometheusModel.Vector)
	if !ok {
		return nil, fmt.Errorf("queryVector returned non-vector response")
	}

	var result []QueryVectorResult
	for _, sample := range r {

		labels := make(map[string]string)
		for name, value := range sample.Metric {
			labels[string(name)] = string(value)
		}

		result = append(result, QueryVectorResult{
			Value:  float64(sample.Value),
			Labels: labels,
		})
	}
	return result, nil
}
