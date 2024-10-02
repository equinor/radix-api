package auth_test

import (
	"context"
	"errors"
	"net/http"
	"os"
	"testing"

	certfake "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned/fake"
	"github.com/equinor/radix-api/api/applications"
	applicationModels "github.com/equinor/radix-api/api/applications/models"
	"github.com/equinor/radix-api/api/buildstatus"
	metricsMock "github.com/equinor/radix-api/api/metrics/mock"
	controllertest "github.com/equinor/radix-api/api/test"
	"github.com/equinor/radix-api/api/test/mock"
	token "github.com/equinor/radix-api/api/utils/token"
	authnmock "github.com/equinor/radix-api/api/utils/token/mock"
	"github.com/equinor/radix-api/internal/config"
	"github.com/equinor/radix-api/models"
	radixhttp "github.com/equinor/radix-common/net/http"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	builders "github.com/equinor/radix-operator/pkg/apis/utils"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

const (
	clusterName     = "AnyClusterName"
	appAliasDNSZone = "app.dev.radix.equinor.com"
	egressIps       = "0.0.0.0"
	subscriptionId  = "12347718-c8f8-4995-bfbb-02655ff1f89c"
)

func setupTest(t *testing.T, validator *authnmock.MockValidatorInterface, buildStatusMock *mock.MockPipelineBadge) (*commontest.Utils, *controllertest.Utils, *kubefake.Clientset, *radixfake.Clientset, *kedafake.Clientset, *prometheusfake.Clientset, *secretproviderfake.Clientset, *certfake.Clientset) {
	// Setup
	ctrl := gomock.NewController(t)
	kubeclient := kubefake.NewSimpleClientset()
	radixclient := radixfake.NewSimpleClientset()
	kedaClient := kedafake.NewSimpleClientset()
	prometheusclient := prometheusfake.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	certClient := certfake.NewSimpleClientset()

	// commonTestUtils is used for creating CRDs
	commonTestUtils := commontest.NewTestUtils(kubeclient, radixclient, kedaClient, secretproviderclient)
	err := commonTestUtils.CreateClusterPrerequisites(clusterName, egressIps, subscriptionId)
	require.NoError(t, err)
	_ = os.Setenv(defaults.ActiveClusternameEnvironmentVariable, clusterName)

	// controllerTestUtils is used for issuing HTTP request and processing responses
	mockPrometheusHandler := metricsMock.NewMockPrometheusHandler(ctrl)
	mockPrometheusHandler.EXPECT().GetUsedResources(gomock.Any(), radixclient, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(&applicationModels.UsedResources{}, nil)

	if buildStatusMock == nil {
		buildStatusMock = mock.NewMockPipelineBadge(ctrl)
		buildStatusMock.EXPECT().GetBadge(gomock.Any(), gomock.Any()).Return([]byte("hello world"), errors.New("error")).AnyTimes()
	}

	if validator == nil {
		validator = authnmock.NewMockValidatorInterface(gomock.NewController(t))
		validator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).Times(1).Return(controllertest.NewTestPrincipal(), nil)
	}

	controllerTestUtils := controllertest.NewTestUtils(
		kubeclient,
		radixclient,
		kedaClient,
		secretproviderclient,
		certClient,
		validator,
		applications.NewApplicationController(
			func(_ context.Context, _ kubernetes.Interface, _ v1.RadixRegistration) (bool, error) {
				return true, nil
			},
			newTestApplicationHandlerFactory(
				config.Config{},
				func(ctx context.Context, kubeClient kubernetes.Interface, namespace string, configMapName string) (bool, error) {
					return true, nil
				},
			),
			mockPrometheusHandler,
		),
		buildstatus.NewBuildStatusController(buildStatusMock),
	)

	return &commonTestUtils, &controllerTestUtils, kubeclient, radixclient, kedaClient, prometheusclient, secretproviderclient, certClient
}

type testApplicationHandlerFactory struct {
	config                  config.Config
	hasAccessToGetConfigMap applications.HasAccessToGetConfigMapFunc
}

func newTestApplicationHandlerFactory(config config.Config, hasAccessToGetConfigMap applications.HasAccessToGetConfigMapFunc) *testApplicationHandlerFactory {
	return &testApplicationHandlerFactory{
		config,
		hasAccessToGetConfigMap,
	}
}

// Create creates a new ApplicationHandler
func (f *testApplicationHandlerFactory) Create(accounts models.Accounts) applications.ApplicationHandler {
	return applications.NewApplicationHandler(accounts, f.config, f.hasAccessToGetConfigMap)
}

func TestGetApplications_AuthenticatedRequestIsOk(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _, _, _, _, _ := setupTest(t, nil, nil)
	_, err := commonTestUtils.ApplyRegistration(builders.ARadixRegistration())
	require.NoError(t, err)

	// Test

	responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/applications")
	response := <-responseChannel

	applications := make([]applicationModels.ApplicationSummary, 0)
	err = controllertest.GetResponseBody(response, &applications)
	require.NoError(t, err)
	assert.Equal(t, 1, len(applications))
}

func TestGetBuildStatus_AnonymousRequestIsOk(t *testing.T) {
	// Setup
	ctrl := gomock.NewController(t)
	mockValidator := authnmock.NewMockValidatorInterface(ctrl)
	mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).Return(token.NewAnonymousPrincipal(), nil).Times(0)
	buildStatusMock := mock.NewMockPipelineBadge(ctrl)
	buildStatusMock.EXPECT().GetBadge(gomock.Any(), gomock.Any()).Return([]byte("hello world"), errors.New("error")).Times(1)
	commonTestUtils, controllerTestUtils, _, _, _, _, _, _ := setupTest(t, mockValidator, buildStatusMock)
	_, err := commonTestUtils.ApplyRegistration(builders.ARadixRegistration().WithName("anyapp"))
	require.NoError(t, err)

	// Test

	responseChannel := controllerTestUtils.ExecuteUnAuthorizedRequest("GET", "/api/v1/applications/anyapp/environments/qa/buildstatus")
	<-responseChannel
	ctrl.Finish() // We expect buildStatusMock to be called 1 time, without auth middleware getting in the way
}

func TestGetApplications_UnauthenticatedIsForbidden(t *testing.T) {
	// Setup
	mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
	mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).Times(0).Return(token.NewAnonymousPrincipal(), radixhttp.ForbiddenError("invalid token"))
	commonTestUtils, controllerTestUtils, _, _, _, _, _, _ := setupTest(t, mockValidator, nil)
	_, err := commonTestUtils.ApplyRegistration(builders.ARadixRegistration())
	require.NoError(t, err)

	// Test

	responseChannel := controllerTestUtils.ExecuteUnAuthorizedRequest("GET", "/api/v1/applications")
	response := <-responseChannel

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestGetApplications_InvalidTokenIsForbidden(t *testing.T) {
	// Setup
	mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
	mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).Times(0).Return(token.NewAnonymousPrincipal(), radixhttp.ForbiddenError("invalid token"))
	commonTestUtils, controllerTestUtils, _, _, _, _, _, _ := setupTest(t, mockValidator, nil)
	_, err := commonTestUtils.ApplyRegistration(builders.ARadixRegistration())
	require.NoError(t, err)

	// Test

	responseChannel := controllerTestUtils.ExecuteUnAuthorizedRequest("GET", "/api/v1/applications")
	response := <-responseChannel

	assert.Equal(t, http.StatusForbidden, response.Code)
}
