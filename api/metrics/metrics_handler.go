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
	GetCpuReqs(ctx context.Context, appName, namespace string) ([]LabeledResults, error)
	GetCpuAvg(ctx context.Context, appName, namespace, duration string) ([]LabeledResults, error)
	GetMemReqs(ctx context.Context, appName, namespace string) ([]LabeledResults, error)
	GetMemMax(ctx context.Context, appName, namespace, duration string) ([]LabeledResults, error)
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

	results, err := pc.client.GetCpuReqs(ctx, appName, namespace)
	if err != nil {
		return nil, err
	}
	for _, result := range results {
		environment, _ := strings.CutPrefix(result.Namespace, appName+"-")
		utilization.SetCpuReqs(environment, result.Component, result.Pod, math.Round(result.Value*1e6)/1e6)
	}

	results, err = pc.client.GetCpuAvg(ctx, appName, namespace, DefaultDuration)
	if err != nil {
		return nil, err
	}
	for _, result := range results {
		environment, _ := strings.CutPrefix(result.Namespace, appName+"-")
		utilization.SetCpuAvg(environment, result.Component, result.Pod, math.Round(result.Value*1e6)/1e6)
	}

	results, err = pc.client.GetMemReqs(ctx, appName, namespace)
	if err != nil {
		return nil, err
	}
	for _, result := range results {
		environment, _ := strings.CutPrefix(result.Namespace, appName+"-")
		utilization.SetMemReqs(environment, result.Component, result.Pod, math.Round(result.Value))
	}

	results, err = pc.client.GetMemMax(ctx, appName, namespace, DefaultDuration)
	if err != nil {
		return nil, err
	}
	for _, result := range results {
		environment, _ := strings.CutPrefix(result.Namespace, appName+"-")
		utilization.SetMemMax(environment, result.Component, result.Pod, math.Round(result.Value))
	}

	return utilization, nil
}
