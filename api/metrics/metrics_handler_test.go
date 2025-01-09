package metrics_test

import (
	"context"
	"testing"

	"github.com/equinor/radix-api/api/metrics"
	"github.com/equinor/radix-api/api/metrics/mock"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	appName1 = "app1"
)

func Test_handler_GetReplicaResourcesUtilization(t *testing.T) {
	scenarios := []struct {
		name    string
		appName string
		envName string
	}{
		{
			name:    "Get utilization in all environments",
			appName: appName1,
		},
		{
			name:    "Get utilization in specific environments",
			appName: appName1,
			envName: "dev",
		},
		{
			name:    "Requested with arguments",
			appName: appName1,
			envName: "dev",
		},
	}
	for _, ts := range scenarios {
		t.Run(ts.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			client := mock.NewMockClient(ctrl)
			expectedNamespace := getExpectedNamespace(ts.appName, ts.envName)

			cpuReqs := []metrics.LabeledResults{
				{Value: 1, Namespace: appName1 + "-dev", Component: "web", Pod: "web-abcd-1"},
				{Value: 2, Namespace: appName1 + "-dev", Component: "web", Pod: "web-abcd-2"},
			}
			cpuAvg := []metrics.LabeledResults{
				{Value: 0.5, Namespace: appName1 + "-dev", Component: "web", Pod: "web-abcd-1"},
				{Value: 0.7, Namespace: appName1 + "-dev", Component: "web", Pod: "web-abcd-2"},
			}
			memReqs := []metrics.LabeledResults{
				{Value: 100, Namespace: appName1 + "-dev", Component: "web", Pod: "web-abcd-1"},
				{Value: 200, Namespace: appName1 + "-dev", Component: "web", Pod: "web-abcd-2"},
			}
			MemMax := []metrics.LabeledResults{
				{Value: 50, Namespace: appName1 + "-dev", Component: "web", Pod: "web-abcd-1"},
				{Value: 100, Namespace: appName1 + "-dev", Component: "web", Pod: "web-abcd-2"},
			}

			client.EXPECT().GetCpuRequests(gomock.Any(), expectedNamespace).Times(1).Return(cpuReqs, nil)
			client.EXPECT().GetCpuAverage(gomock.Any(), expectedNamespace, "24h").Times(1).Return(cpuAvg, nil)
			client.EXPECT().GetMemoryRequests(gomock.Any(), expectedNamespace).Times(1).Return(memReqs, nil)
			client.EXPECT().GetMemoryMaximum(gomock.Any(), expectedNamespace, "24h").Times(1).Return(MemMax, nil)

			metricsHandler := metrics.NewHandler(client)
			response, err := metricsHandler.GetReplicaResourcesUtilization(context.Background(), appName1, ts.envName)
			assert.NoError(t, err)

			require.NotNil(t, response)
			require.Contains(t, response.Environments, "dev")
			require.Contains(t, response.Environments["dev"].Components, "web")
			assert.Contains(t, response.Environments["dev"].Components["web"].Replicas, "web-abcd-1")
			assert.Contains(t, response.Environments["dev"].Components["web"].Replicas, "web-abcd-2")

			assert.EqualValues(t, 1, response.Environments["dev"].Components["web"].Replicas["web-abcd-1"].CpuRequests)
			assert.EqualValues(t, 0.5, response.Environments["dev"].Components["web"].Replicas["web-abcd-1"].CpuAverage)
			assert.EqualValues(t, 100, response.Environments["dev"].Components["web"].Replicas["web-abcd-1"].MemoryRequests)
			assert.EqualValues(t, 50, response.Environments["dev"].Components["web"].Replicas["web-abcd-1"].MemoryMaximum)

			assert.EqualValues(t, 2, response.Environments["dev"].Components["web"].Replicas["web-abcd-2"].CpuRequests)
			assert.EqualValues(t, 0.7, response.Environments["dev"].Components["web"].Replicas["web-abcd-2"].CpuAverage)
			assert.EqualValues(t, 200, response.Environments["dev"].Components["web"].Replicas["web-abcd-2"].MemoryRequests)
			assert.EqualValues(t, 100, response.Environments["dev"].Components["web"].Replicas["web-abcd-2"].MemoryMaximum)

			assert.NotEmpty(t, response)
		})
	}
}

func getExpectedNamespace(appName, envName string) string {
	if envName == "" {
		return appName + "-.*"
	}
	return appName + "-" + envName
}
