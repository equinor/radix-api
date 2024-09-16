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
		envName       string
		componentName string
		duration      string
		since         string
		ignoreZero    bool
	}

	type scenario struct {
		name                  string
		args                  args
		expectedUsedResources *applicationModels.UsedResources
		expectedMetrics       map[QueryName]model.Value
		expectedWarnings      []string
		expectedError         error
	}

	scenarios := []scenario{
		{
			name: "Test GetUsedResources", args: args{}},
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
				Return(ts.expectedMetrics, ts.expectedWarnings, ts.expectedError)

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
