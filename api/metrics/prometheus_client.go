package metrics

import (
	"context"
	"errors"
	"time"

	prometheusApi "github.com/prometheus/client_golang/api"
	prometheusV1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	prometheusModel "github.com/prometheus/common/model"
	"github.com/rs/zerolog/log"
)

// PrometheusClient Interface for Prometheus client
type PrometheusClient interface {
	// GetMetrics Get metrics for the application
	GetMetrics(ctx context.Context, appName, envName, componentName, duration, since string) (map[QueryName]prometheusModel.Value, []string, error)
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
func (c *client) GetMetrics(ctx context.Context, appName, envName, componentName, duration, since string) (map[QueryName]prometheusModel.Value, []string, error) {
	results := make(map[QueryName]model.Value)
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
