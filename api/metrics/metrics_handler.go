package metrics

import (
	"context"
	"math"
	"regexp"

	applicationModels "github.com/equinor/radix-api/api/applications/models"
)

const (
	DefaultDuration = "24h"
)

type LabeledResults struct {
	Value       float64
	Environment string
	Component   string
	Pod         string
}
type Client interface {
	// GetCpuRequests returns a list of all pods with their CPU requets. The envName can be empty to return all environments. It will return the labels label_radix_component, namespace, pod and container.
	GetCpuRequests(ctx context.Context, appName, envName string) ([]LabeledResults, error)
	// GetCpuAverage returns a list of all pods with their average CPU usage. The envName can be empty to return all environments. It will return the labels label_radix_component, namespace, pod and container.
	GetCpuAverage(ctx context.Context, appName, envName, duration string) ([]LabeledResults, error)
	// GetMemoryRequests returns a list of all pods with their Memory requets. The envName can be empty to return all environments. It will return the labels label_radix_component, namespace, pod and container.
	GetMemoryRequests(ctx context.Context, appName, envName string) ([]LabeledResults, error)
	// GetMemoryMaximum returns a list of all pods with their maximum Memory usage. The envName can be empty to return all environments. It will return the labels label_radix_component, namespace, pod and container.
	GetMemoryMaximum(ctx context.Context, appName, envName, duration string) ([]LabeledResults, error)
}

type Handler struct {
	client Client
}

// NewHandler Constructor for Prometheus handler
func NewHandler(client Client) *Handler {
	return &Handler{
		client: client,
	}
}

// GetReplicaResourcesUtilization Get used resources for the application. envName is optional. Will fallback to all copmonent environments to the application.
func (pc *Handler) GetReplicaResourcesUtilization(ctx context.Context, appName, envName string) (*applicationModels.ReplicaResourcesUtilizationResponse, error) {
	utilization := applicationModels.NewPodResourcesUtilizationResponse()
	appName = regexp.QuoteMeta(appName)
	envName = regexp.QuoteMeta(envName)

	results, err := pc.client.GetCpuRequests(ctx, appName, envName)
	if err != nil {
		return nil, err
	}
	for _, result := range results {
		utilization.SetCpuRequests(result.Environment, result.Component, result.Pod, math.Round(result.Value*1e6)/1e6)
	}

	results, err = pc.client.GetCpuAverage(ctx, appName, envName, DefaultDuration)
	if err != nil {
		return nil, err
	}
	for _, result := range results {
		utilization.SetCpuAverage(result.Environment, result.Component, result.Pod, math.Round(result.Value*1e6)/1e6)
	}

	results, err = pc.client.GetMemoryRequests(ctx, appName, envName)
	if err != nil {
		return nil, err
	}
	for _, result := range results {
		utilization.SetMemoryRequests(result.Environment, result.Component, result.Pod, math.Round(result.Value))
	}

	results, err = pc.client.GetMemoryMaximum(ctx, appName, envName, DefaultDuration)
	if err != nil {
		return nil, err
	}
	for _, result := range results {
		utilization.SetMemoryMaximum(result.Environment, result.Component, result.Pod, math.Round(result.Value))
	}

	return utilization, nil
}
