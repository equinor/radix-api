package metrics_test

import (
	"context"
	"testing"

	"github.com/equinor/radix-api/api/metrics"
	"github.com/equinor/radix-api/api/metrics/mock"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
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

			client.EXPECT().GetCpuReqs(gomock.Any(), ts.appName, expectedNamespace).Times(1).Return([]metrics.LabeledResults{}, nil)
			client.EXPECT().GetCpuAvg(gomock.Any(), ts.appName, expectedNamespace, "24h").Times(1).Return([]metrics.LabeledResults{}, nil)
			client.EXPECT().GetMemReqs(gomock.Any(), ts.appName, expectedNamespace).Times(1).Return([]metrics.LabeledResults{}, nil)
			client.EXPECT().GetMemMax(gomock.Any(), ts.appName, expectedNamespace, "24h").Times(1).Return([]metrics.LabeledResults{}, nil)

			metricsHandler := metrics.NewHandler(client)
			_, err := metricsHandler.GetReplicaResourcesUtilization(context.Background(), appName1, ts.envName)
			assert.NoError(t, err)
		})
	}
}

func getExpectedNamespace(appName, envName string) string {
	if envName == "" {
		return appName + "-.*"
	}
	return appName + "-" + envName
}
