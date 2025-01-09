package prometheus

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/equinor/radix-api/api/metrics"
	"github.com/equinor/radix-api/api/utils/logs"
	prometheusApi "github.com/prometheus/client_golang/api"
	prometheusV1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type QueryAPI interface {
	Query(ctx context.Context, query string, ts time.Time, opts ...prometheusV1.Option) (model.Value, prometheusV1.Warnings, error)
}

type Client struct {
	api QueryAPI
}

// NewPrometheusClient Constructor for a Prometheus Metrics Client
func NewPrometheusClient(prometheusUrl string) (metrics.Client, error) {
	logger := logs.NewRoundtripLogger(func(e *zerolog.Event) {
		e.Str("PrometheusClient", "prometheus")
	})

	apiClient, err := prometheusApi.NewClient(prometheusApi.Config{Address: prometheusUrl, RoundTripper: logger(prometheusApi.DefaultRoundTripper)})
	if err != nil {
		return nil, errors.New("failed to create the Prometheus API PrometheusClient")
	}
	api := prometheusV1.NewAPI(apiClient)

	return NewClient(api), nil
}

func NewClient(api QueryAPI) metrics.Client {
	return &Client{api: api}
}

// GetCpuRequests returns a list of all pods with their CPU requets. The Namespace can be regex. It will return the labels label_radix_component, namespace, pod and container.
func (c *Client) GetCpuRequests(ctx context.Context, namespace string) ([]metrics.LabeledResults, error) {
	namespace = escapeVariable(namespace)
	return c.queryVector(ctx, fmt.Sprintf(`max by(namespace, container, pod) (kube_pod_container_resource_requests{container!="", namespace=~"%s",resource="cpu"}) * on(pod) group_left(label_radix_component) kube_pod_labels{label_radix_component!=""}`, namespace))
}

// GetCpuAverage returns a list of all pods with their average CPU usage. The Namespace can be regex. It will return the labels label_radix_component, namespace, pod and container.
func (c *Client) GetCpuAverage(ctx context.Context, namespace, duration string) ([]metrics.LabeledResults, error) {
	namespace = escapeVariable(namespace)
	return c.queryVector(ctx, fmt.Sprintf(`max by(namespace, container, pod) (avg_over_time(irate(container_cpu_usage_seconds_total{container!="", namespace=~"%s"}[1m]) [%s:])) * on(pod) group_left(label_radix_component) kube_pod_labels{label_radix_component!=""}`, namespace, duration))
}

// GetMemoryRequests returns a list of all pods with their Memory requets. The Namespace can be regex. It will return the labels label_radix_component, namespace, pod and container.
func (c *Client) GetMemoryRequests(ctx context.Context, namespace string) ([]metrics.LabeledResults, error) {
	namespace = escapeVariable(namespace)
	return c.queryVector(ctx, fmt.Sprintf(`max by(namespace, container, pod) (kube_pod_container_resource_requests{container!="", namespace=~"%s",resource="memory"}) * on(pod) group_left(label_radix_component) kube_pod_labels{label_radix_component!=""}`, namespace))
}

// GetMemoryMaximum returns a list of all pods with their maximum Memory usage. The Namespace can be regex. It will return the labels label_radix_component, namespace, pod and container.
func (c *Client) GetMemoryMaximum(ctx context.Context, namespace, duration string) ([]metrics.LabeledResults, error) {
	namespace = escapeVariable(namespace)
	return c.queryVector(ctx, fmt.Sprintf(`max by(namespace, container, pod) (max_over_time(container_memory_usage_bytes{container!="", namespace=~"%s"} [%s:])) * on(pod) group_left(label_radix_component) kube_pod_labels{label_radix_component!=""}`, namespace, duration))
}

func (c *Client) queryVector(ctx context.Context, query string) ([]metrics.LabeledResults, error) {
	response, w, err := c.api.Query(ctx, query, time.Now())
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Str("query", query).Msg("fetching vector query")
		return nil, err
	}
	if len(w) > 0 {
		log.Ctx(ctx).Warn().Str("query", query).Strs("warnings", w).Msgf("fetching vector query")
	} else {
		log.Ctx(ctx).Trace().Str("query", query).Msgf("fetching vector query")
	}

	r, ok := response.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("queryVector returned non-vector response")
	}

	var result []metrics.LabeledResults
	for _, sample := range r {
		result = append(result, metrics.LabeledResults{
			Value:     float64(sample.Value),
			Namespace: string(sample.Metric["namespace"]),
			Component: string(sample.Metric["label_radix_component"]),
			Pod:       string(sample.Metric["pod"]),
		})
	}
	return result, nil
}

func escapeVariable(namespace string) string {
	namespace = strings.Replace(namespace, `"`, `\"`, -1)
	return namespace
}
