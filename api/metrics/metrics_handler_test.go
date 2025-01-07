package metrics_test

import (
	"context"
	"testing"

	"github.com/equinor/radix-api/api/metrics"
	"github.com/equinor/radix-api/api/metrics/prometheus"
	prometheusMock "github.com/equinor/radix-api/api/metrics/prometheus/mock"
	"github.com/golang/mock/gomock"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
)

type args struct {
	appName string
	envName string
}

const (
	appName1 = "app1"
)

func Test_handler_GetReplicaResourcesUtilization(t *testing.T) {
	scenarios := []struct {
		name string
		args args
	}{
		{
			name: "Get utilization in all environments",
			args: args{
				appName: appName1,
			},
		},
		{
			name: "Get utilization in specific environments",
			args: args{
				appName: appName1,
				envName: "dev",
			},
		},
		{
			name: "Requested with arguments",
			args: args{
				appName: appName1,
				envName: "dev",
			},
		},
	}
	for _, ts := range scenarios {
		t.Run(ts.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			api := prometheusMock.NewMockQueryAPI(ctrl)

			api.EXPECT().Query(gomock.Any(), gomock.Any(), gomock.Any()).Times(4).Return(model.Vector{}, nil, nil)

			client := prometheus.NewClient(api)

			assert.Fail(t, "Test arguements are used correctly")

			metricsHandler := metrics.NewHandler(client)
			_, err := metricsHandler.GetReplicaResourcesUtilization(context.Background(), appName1, ts.args.envName)
			assert.NoError(t, err)
		})
	}
}
