package metrics

import (
	"context"
	"reflect"
	"testing"

	applicationModels "github.com/equinor/radix-api/api/applications/models"
	"github.com/equinor/radix-api/api/metrics/mock"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	builders "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func Test_handler_GetUsedResources(t *testing.T) {
	const (
		appName1 = "app1"
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
				appName: appName1,
			},
			clientReturnsMetrics: map[QueryName]model.Value{
				cpuMax: model.Vector{&model.Sample{Metric: map[model.LabelName]model.LabelValue{}, Value: 1}},
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
			mockPrometheusClient := mock.NewMockPrometheusClient(ctrl)
			mockPrometheusClient.EXPECT().GetMetrics(gomock.Any(), appName1, ts.args.envName, ts.args.componentName, ts.args.duration, ts.args.since).
				Return(ts.clientReturnsMetrics, ts.expectedWarnings, ts.expectedError)

			ph := &handler{
				client: mockPrometheusClient,
			}
			got, err := ph.GetUsedResources(context.Background(), radixClient, appName1, ts.args.envName, ts.args.componentName, ts.args.duration, ts.args.since, ts.args.ignoreZero)
			if err != ts.expectedError {
				t.Errorf("GetUsedResources() error = %v, expectedError %v", err, ts.expectedError)
				return
			}
			if !reflect.DeepEqual(got, ts.expectedUsedResources) {
				t.Errorf("GetUsedResources() got = %v, expectedUsedResources %v", got, ts.expectedUsedResources)
			}
		})
	}
}
