package metrics

import (
	"context"
	"math"
	"strings"

	applicationModels "github.com/equinor/radix-api/api/applications/models"
)

const (
	DefaultDuration = "24h"
)

type LabeledResults struct {
	Value     float64
	Namespace string
	Component string
	Pod       string
}
type Client interface {
	GetCpuRequests(ctx context.Context, namespace string) ([]LabeledResults, error)
	GetCpuAverage(ctx context.Context, namespace, duration string) ([]LabeledResults, error)
	GetMemoryRequests(ctx context.Context, namespace string) ([]LabeledResults, error)
	GetMemoryMaximum(ctx context.Context, namespace, duration string) ([]LabeledResults, error)
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
	namespace := appName + "-.*"
	if envName != "" {
		namespace = appName + "-" + envName
	}

	extractEnv := func(namespace string) string {
		env, _ := strings.CutPrefix(namespace, appName+"-")
		return env
	}

	results, err := pc.client.GetCpuRequests(ctx, namespace)
	if err != nil {
		return nil, err
	}
	for _, result := range results {
		utilization.SetCpuRequests(extractEnv(result.Namespace), result.Component, result.Pod, math.Round(result.Value*1e6)/1e6)
	}

	results, err = pc.client.GetCpuAverage(ctx, namespace, DefaultDuration)
	if err != nil {
		return nil, err
	}
	for _, result := range results {
		utilization.SetCpuAverage(extractEnv(result.Namespace), result.Component, result.Pod, math.Round(result.Value*1e6)/1e6)
	}

	results, err = pc.client.GetMemoryRequests(ctx, namespace)
	if err != nil {
		return nil, err
	}
	for _, result := range results {
		utilization.SetMemoryRequests(extractEnv(result.Namespace), result.Component, result.Pod, math.Round(result.Value))
	}

	results, err = pc.client.GetMemoryMaximum(ctx, namespace, DefaultDuration)
	if err != nil {
		return nil, err
	}
	for _, result := range results {
		utilization.SetMemoryMaximum(extractEnv(result.Namespace), result.Component, result.Pod, math.Round(result.Value))
	}

	return utilization, nil
}
