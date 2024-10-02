package metrics

import (
	"context"
	"errors"
	"testing"

	applicationModels "github.com/equinor/radix-api/api/applications/models"
	"github.com/equinor/radix-api/api/metrics/internal"
	"github.com/equinor/radix-api/api/metrics/mock"
	"github.com/equinor/radix-common/utils/pointers"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	builders "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type args struct {
	appName       string
	envName       string
	componentName string
	duration      string
	since         string
}

type scenario struct {
	name                  string
	args                  args
	clientReturnsMetrics  map[internal.QueryName]model.Value
	expectedUsedResources *applicationModels.UsedResources
	expectedWarnings      []string
	expectedError         error
}

const (
	appName1            = "app1"
	metricsKeyContainer = "container"
	metricsKeyNamespace = "namespace"
)

func Test_handler_GetUsedResources(t *testing.T) {
	scenarios := []scenario{
		{
			name: "Got used resources",
			args: args{
				appName:  appName1,
				duration: defaultDuration,
			},
			clientReturnsMetrics:  getClientReturnsMetrics(),
			expectedUsedResources: getExpectedUsedResources(),
		},
		{
			name: "Got used resources with warnings",
			args: args{
				appName:  appName1,
				duration: defaultDuration,
			},
			clientReturnsMetrics:  getClientReturnsMetrics(),
			expectedUsedResources: getExpectedUsedResources("Warning1", "Warning2"),
			expectedWarnings:      []string{"Warning1", "Warning2"},
		},
		{
			name: "Requested with arguments",
			args: args{
				appName:       appName1,
				envName:       "dev",
				componentName: "component1",
				duration:      defaultDuration,
				since:         "2d",
			},
			clientReturnsMetrics:  getClientReturnsMetrics(),
			expectedUsedResources: getExpectedUsedResources(),
		},
		{
			name: "With error",
			args: args{
				appName:  appName1,
				duration: defaultDuration,
			},
			expectedError: errors.New("failed to get Prometheus metrics"),
		},
	}
	for _, ts := range scenarios {
		t.Run(ts.name, func(t *testing.T) {
			radixClient := fake.NewSimpleClientset()
			commonTestUtils := commontest.NewTestUtils(nil, radixClient, nil, nil)
			_, err := commonTestUtils.ApplyRegistration(builders.ARadixRegistration().WithName(appName1))
			require.NoError(t, err)
			ctrl := gomock.NewController(t)
			mockPrometheusClient := mock.NewMockPrometheusClient(ctrl)
			mockPrometheusClient.EXPECT().GetMetrics(gomock.Any(), appName1, ts.args.envName, ts.args.componentName, ts.args.duration, ts.args.since).
				Return(ts.clientReturnsMetrics, ts.expectedWarnings, ts.expectedError)

			prometheusHandler := &handler{
				client: mockPrometheusClient,
			}
			got, err := prometheusHandler.GetUsedResources(context.Background(), radixClient, appName1, ts.args.envName, ts.args.componentName, ts.args.duration, ts.args.since)
			if ts.expectedError != nil {
				assert.ErrorIs(t, err, ts.expectedError, "Missing or not matching GetUsedResources() error")
				return
			} else {
				require.NoError(t, err, "Missing or not matching GetUsedResources() error")
			}
			assertExpected(t, ts, got)
		})
	}
}

func assertExpected(t *testing.T, ts scenario, got *applicationModels.UsedResources) {
	assert.ElementsMatch(t, ts.expectedWarnings, got.Warnings, "Warnings")
	assert.NotNil(t, got.CPU.Min, "nil CPU.Min")
	assert.NotNil(t, got.CPU.Max, "nil CPU.Max")
	assert.NotNil(t, *got.CPU.Avg, "nil CPU.Avg")
	assert.Equal(t, *ts.expectedUsedResources.CPU.Min, *got.CPU.Min, "CPU.Min")
	assert.Equal(t, *ts.expectedUsedResources.CPU.Max, *got.CPU.Max, "CPU.Max")
	assert.Equal(t, *ts.expectedUsedResources.CPU.Avg, *got.CPU.Avg, "CPU.Avg")
	assert.NotNil(t, got.Memory.Min, "nil Memory.Min")
	assert.NotNil(t, got.Memory.Max, "nil Memory.Max")
	assert.NotNil(t, got.Memory.Avg, "nil Memory.Avg")
	assert.Equal(t, *ts.expectedUsedResources.Memory.Min, *got.Memory.Min, "Memory.Min")
	assert.Equal(t, *ts.expectedUsedResources.Memory.Max, *got.Memory.Max, "Memory.Max")
	assert.Equal(t, *ts.expectedUsedResources.Memory.Avg, *got.Memory.Avg, "Memory.Avg")
	assert.NotEmpty(t, got.From, "From")
	assert.NotEmpty(t, got.To, "To")
}

func getClientReturnsMetrics() map[internal.QueryName]model.Value {
	return map[internal.QueryName]model.Value{
		internal.CpuMax: model.Vector{
			&model.Sample{Metric: map[model.LabelName]model.LabelValue{metricsKeyContainer: "server", metricsKeyNamespace: "app-dev"}, Value: 0.008123134},
			&model.Sample{Metric: map[model.LabelName]model.LabelValue{metricsKeyContainer: "server", metricsKeyNamespace: "app-prod"}, Value: 0.126576764},
		},
		internal.CpuAvg: model.Vector{
			&model.Sample{Metric: map[model.LabelName]model.LabelValue{metricsKeyContainer: "server", metricsKeyNamespace: "app-dev"}, Value: 0.0023213546},
			&model.Sample{Metric: map[model.LabelName]model.LabelValue{metricsKeyContainer: "server", metricsKeyNamespace: "app-prod"}, Value: 0.047546577},
		},
		internal.CpuMin: model.Vector{
			&model.Sample{Metric: map[model.LabelName]model.LabelValue{metricsKeyContainer: "server", metricsKeyNamespace: "app-dev"}, Value: 0.0019874},
			&model.Sample{Metric: map[model.LabelName]model.LabelValue{metricsKeyContainer: "server", metricsKeyNamespace: "app-prod"}, Value: 0.02321456},
		},
		internal.MemoryMax: model.Vector{
			&model.Sample{Metric: map[model.LabelName]model.LabelValue{metricsKeyContainer: "server", metricsKeyNamespace: "app-dev"}, Value: 123456.3475613},
			&model.Sample{Metric: map[model.LabelName]model.LabelValue{metricsKeyContainer: "server", metricsKeyNamespace: "app-prod"}, Value: 234567.34575412},
		},
		internal.MemoryAvg: model.Vector{
			&model.Sample{Metric: map[model.LabelName]model.LabelValue{metricsKeyContainer: "server", metricsKeyNamespace: "app-dev"}, Value: 90654.81},
			&model.Sample{Metric: map[model.LabelName]model.LabelValue{metricsKeyContainer: "server", metricsKeyNamespace: "app-prod"}, Value: 150654.12398771},
		},
		internal.MemoryMin: model.Vector{
			&model.Sample{Metric: map[model.LabelName]model.LabelValue{metricsKeyContainer: "server", metricsKeyNamespace: "app-dev"}, Value: 56731.2324654},
			&model.Sample{Metric: map[model.LabelName]model.LabelValue{metricsKeyContainer: "server", metricsKeyNamespace: "app-prod"}, Value: 112234.456789},
		},
	}
}

func getExpectedUsedResources(warnings ...string) *applicationModels.UsedResources {
	return &applicationModels.UsedResources{
		Warnings: warnings,
		CPU: &applicationModels.UsedResource{
			Min: pointers.Ptr(0.025202),
			Avg: pointers.Ptr(0.049868),
			Max: pointers.Ptr(0.1347),
		},
		Memory: &applicationModels.UsedResource{
			Min: pointers.Ptr(168966.0),
			Avg: pointers.Ptr(241309.0),
			Max: pointers.Ptr(358024.0),
		},
	}
}
