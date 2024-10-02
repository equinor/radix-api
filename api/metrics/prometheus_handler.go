package metrics

import (
	"context"
	"math"
	"regexp"
	"time"

	applicationModels "github.com/equinor/radix-api/api/applications/models"
	"github.com/equinor/radix-api/api/metrics/internal"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/prometheus/common/model"
	prometheusModel "github.com/prometheus/common/model"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	durationExpression = `^[0-9]{1,5}[mhdw]$`
	defaultDuration    = "30d"
	defaultOffset      = ""
)

// PrometheusHandler Interface for Prometheus handler
type PrometheusHandler interface {
	GetUsedResources(ctx context.Context, radixClient radixclient.Interface, appName, envName, componentName, duration, since string) (*applicationModels.UsedResources, error)
}

type handler struct {
	client PrometheusClient
}

// NewPrometheusHandler Constructor for Prometheus handler
func NewPrometheusHandler(client PrometheusClient) PrometheusHandler {
	return &handler{
		client: client,
	}
}

// GetUsedResources Get used resources for the application
func (pc *handler) GetUsedResources(ctx context.Context, radixClient radixclient.Interface, appName, envName, componentName, duration, since string) (*applicationModels.UsedResources, error) {
	_, err := radixClient.RadixV1().RadixRegistrations().Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	durationValue, duration, err := parseQueryDuration(duration, defaultDuration)
	if err != nil {
		return nil, err
	}
	sinceValue, since, err := parseQueryDuration(since, defaultOffset)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	log.Ctx(ctx).Debug().Msgf("Getting used resources for application %s", appName)
	results, warnings, err := pc.client.GetMetrics(ctx, appName, envName, componentName, duration, since)
	if err != nil {
		return nil, err
	}
	resources := getUsedResourcesByMetrics(ctx, results, durationValue, sinceValue)
	resources.Warnings = warnings
	log.Ctx(ctx).Debug().Msgf("Got used resources for application %s", appName)
	return resources, nil
}

func getUsedResourcesByMetrics(ctx context.Context, results map[internal.QueryName]prometheusModel.Value, queryDuration time.Duration, querySince time.Duration) *applicationModels.UsedResources {
	usedCpuResource := applicationModels.UsedResource{}
	usedCpuResource.Min = getCpuMetricValue(ctx, results, internal.CpuMin)
	usedCpuResource.Max = getCpuMetricValue(ctx, results, internal.CpuMax)
	usedCpuResource.Avg = getCpuMetricValue(ctx, results, internal.CpuAvg)
	usedMemoryResource := applicationModels.UsedResource{}
	usedMemoryResource.Min = getMemoryMetricValue(ctx, results, internal.MemoryMin)
	usedMemoryResource.Max = getMemoryMetricValue(ctx, results, internal.MemoryMax)
	usedMemoryResource.Avg = getMemoryMetricValue(ctx, results, internal.MemoryAvg)
	now := time.Now()
	return &applicationModels.UsedResources{
		From:   radixutils.FormatTimestamp(now.Add(-queryDuration)),
		To:     radixutils.FormatTimestamp(now.Add(-querySince)),
		CPU:    &usedCpuResource,
		Memory: &usedMemoryResource,
	}
}

func parseQueryDuration(duration string, defaultValue string) (time.Duration, string, error) {
	if len(duration) == 0 || !regexp.MustCompile(durationExpression).MatchString(duration) {
		duration = defaultValue
	}
	if len(duration) == 0 {
		return 0, duration, nil
	}
	parsedDuration, err := prometheusModel.ParseDuration(duration)
	return time.Duration(parsedDuration), duration, err
}

func getCpuMetricValue(ctx context.Context, queryResults map[internal.QueryName]prometheusModel.Value, queryName internal.QueryName) *float64 {
	if value, ok := getMetricsValue(ctx, queryResults, queryName); ok {
		return pointers.Ptr(math.Round(value*1e6) / 1e6)
	}
	return nil
}

func getMemoryMetricValue(ctx context.Context, queryResults map[internal.QueryName]prometheusModel.Value, queryName internal.QueryName) *float64 {
	if value, ok := getMetricsValue(ctx, queryResults, queryName); ok {
		return pointers.Ptr(math.Round(value))
	}
	return nil
}

func getMetricsValue(ctx context.Context, queryResults map[internal.QueryName]prometheusModel.Value, queryName internal.QueryName) (float64, bool) {
	queryResult, ok := queryResults[queryName]
	if !ok {
		return 0.0, false
	}
	groupedMetrics, ok := queryResult.(model.Vector)
	if !ok {
		log.Ctx(ctx).Error().Msgf("Failed to convert metrics query %s result to Vector", queryName)
		return 0, false
	}
	sum := 0.0
	for _, sample := range groupedMetrics {
		sum += float64(sample.Value)
	}
	return sum, true
}
