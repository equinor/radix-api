package metrics

import (
	"context"
	"errors"
	"testing"

	applicationModels "github.com/equinor/radix-api/api/applications/models"
	"github.com/equinor/radix-common/utils/pointers"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	builders "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_handler_GetUsedResources(t *testing.T) {
	const (
		appName1            = "app1"
		metricsKeyContainer = "container"
		metricsKeyNamespace = "namespace"
	)

	type args struct {
		appName       string
		envName       string
		componentName string
		duration      string
		since         string
		ignoreZero    bool
	}

	type scenario struct {
		name                  string
		args                  args
		clientReturnsMetrics  map[QueryName]model.Value
		expectedUsedResources *applicationModels.UsedResources
		expectedWarnings      []string
		expectedError         error
	}

	scenarios := []scenario{
		{
			name: "Got used resources",
			args: args{
				appName:  appName1,
				duration: defaultDuration,
			},
			clientReturnsMetrics: map[QueryName]model.Value{
				cpuMax: model.Vector{
					&model.Sample{Metric: map[model.LabelName]model.LabelValue{metricsKeyContainer: "server", metricsKeyNamespace: "app-dev"}, Value: 0.008123134},
					&model.Sample{Metric: map[model.LabelName]model.LabelValue{metricsKeyContainer: "server", metricsKeyNamespace: "app-prod"}, Value: 0.126576764},
				},
				cpuAvg: model.Vector{
					&model.Sample{Metric: map[model.LabelName]model.LabelValue{metricsKeyContainer: "server", metricsKeyNamespace: "app-dev"}, Value: 0.0023213546},
					&model.Sample{Metric: map[model.LabelName]model.LabelValue{metricsKeyContainer: "server", metricsKeyNamespace: "app-prod"}, Value: 0.047546577},
				},
				cpuMin: model.Vector{
					&model.Sample{Metric: map[model.LabelName]model.LabelValue{metricsKeyContainer: "server", metricsKeyNamespace: "app-dev"}, Value: 0.0019874},
					&model.Sample{Metric: map[model.LabelName]model.LabelValue{metricsKeyContainer: "server", metricsKeyNamespace: "app-prod"}, Value: 0.02321456},
				},
				memoryMax: model.Vector{
					&model.Sample{Metric: map[model.LabelName]model.LabelValue{metricsKeyContainer: "server", metricsKeyNamespace: "app-dev"}, Value: 123456.3475613},
					&model.Sample{Metric: map[model.LabelName]model.LabelValue{metricsKeyContainer: "server", metricsKeyNamespace: "app-prod"}, Value: 234567.34575412},
				},
				memoryAvg: model.Vector{
					&model.Sample{Metric: map[model.LabelName]model.LabelValue{metricsKeyContainer: "server", metricsKeyNamespace: "app-dev"}, Value: 90654.81},
					&model.Sample{Metric: map[model.LabelName]model.LabelValue{metricsKeyContainer: "server", metricsKeyNamespace: "app-prod"}, Value: 150654.12398771},
				},
				memoryMin: model.Vector{
					&model.Sample{Metric: map[model.LabelName]model.LabelValue{metricsKeyContainer: "server", metricsKeyNamespace: "app-dev"}, Value: 56731.2324654},
					&model.Sample{Metric: map[model.LabelName]model.LabelValue{metricsKeyContainer: "server", metricsKeyNamespace: "app-prod"}, Value: 112234.456789},
				},
			},
			expectedUsedResources: &applicationModels.UsedResources{
				CPU: &applicationModels.UsedResource{
					Min:       "1m",
					Max:       "126m",
					Average:   "24m",
					MinActual: pointers.Ptr(1.9874),
					MaxActual: pointers.Ptr(126.576764),
					AvgActual: pointers.Ptr(24.933966),
				},
				Memory: &applicationModels.UsedResource{
					Min:       "56M",
					Max:       "234M",
					Average:   "120M",
					MinActual: pointers.Ptr(56.731232),
					MaxActual: pointers.Ptr(234.567346),
					AvgActual: pointers.Ptr(120.654467),
				},
			},
		},
	}
	for _, ts := range scenarios {
		t.Run(ts.name, func(t *testing.T) {
			radixClient := fake.NewSimpleClientset()
			commonTestUtils := commontest.NewTestUtils(nil, radixClient, nil, nil)
			_, err := commonTestUtils.ApplyRegistration(builders.ARadixRegistration().WithName(appName1))
			require.NoError(t, err)
			ctrl := gomock.NewController(t)
			mockPrometheusClient := NewMockPrometheusClient(ctrl)
			mockPrometheusClient.EXPECT().GetMetrics(gomock.Any(), appName1, ts.args.envName, ts.args.componentName, ts.args.duration, ts.args.since).
				Return(ts.clientReturnsMetrics, ts.expectedWarnings, ts.expectedError)

			ph := &handler{
				client: mockPrometheusClient,
			}
			got, err := ph.GetUsedResources(context.Background(), radixClient, appName1, ts.args.envName, ts.args.componentName, ts.args.duration, ts.args.since, ts.args.ignoreZero)
			if !errors.Is(ts.expectedError, err) {
				t.Errorf("GetUsedResources() error = %v, expectedError %v", err, ts.expectedError)
				return
			}
			assert.ElementsMatch(t, ts.expectedWarnings, got.Warnings, "Warnings")
			assert.Equal(t, ts.expectedUsedResources.CPU.Min, got.CPU.Min, "CPU.Min")
			assert.Equal(t, ts.expectedUsedResources.CPU.Max, got.CPU.Max, "CPU.Max")
			assert.Equal(t, ts.expectedUsedResources.CPU.Average, got.CPU.Average, "CPU.Average")
			assert.NotNil(t, got.CPU.MinActual, "nil CPU.MinActual")
			assert.NotNil(t, got.CPU.MaxActual, "nil CPU.MaxActual")
			assert.NotNil(t, *got.CPU.AvgActual, "nil CPU.AvgActual")
			assert.Equal(t, *ts.expectedUsedResources.CPU.MinActual, *got.CPU.MinActual, "CPU.MinActual")
			assert.Equal(t, *ts.expectedUsedResources.CPU.MaxActual, *got.CPU.MaxActual, "CPU.MaxActual")
			assert.Equal(t, *ts.expectedUsedResources.CPU.AvgActual, *got.CPU.AvgActual, "CPU.AvgActual")
			assert.Equal(t, ts.expectedUsedResources.Memory.Min, got.Memory.Min, "Memory.Min")
			assert.Equal(t, ts.expectedUsedResources.Memory.Max, got.Memory.Max, "Memory.Max")
			assert.Equal(t, ts.expectedUsedResources.Memory.Average, got.Memory.Average, "Memory.Average")
			assert.NotNil(t, got.Memory.MinActual, "nil Memory.MinActual")
			assert.NotNil(t, got.Memory.MaxActual, "nil Memory.MaxActual")
			assert.NotNil(t, got.Memory.AvgActual, "nil Memory.AvgActual")
			assert.Equal(t, *ts.expectedUsedResources.Memory.MinActual, *got.Memory.MinActual, "Memory.MinActual")
			assert.Equal(t, *ts.expectedUsedResources.Memory.MaxActual, *got.Memory.MaxActual, "Memory.MaxActual")
			assert.Equal(t, *ts.expectedUsedResources.Memory.AvgActual, *got.Memory.AvgActual, "Memory.AvgActual")
			assert.NotEmpty(t, got.From, "From")
			assert.NotEmpty(t, got.To, "To")
		})
	}
}
