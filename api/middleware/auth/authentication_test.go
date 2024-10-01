package auth_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	certfake "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned/fake"
	"github.com/equinor/radix-api/api/applications"
	applicationModels "github.com/equinor/radix-api/api/applications/models"
	controllertest "github.com/equinor/radix-api/api/test"
	token "github.com/equinor/radix-api/api/utils/authn"
	authnmock "github.com/equinor/radix-api/api/utils/authn/mock"
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

func setupTest(t *testing.T, requireAppConfigurationItem, requireAppADGroups bool) (*commontest.Utils, *controllertest.Utils, *kubefake.Clientset, *radixfake.Clientset, *kedafake.Clientset, *prometheusfake.Clientset, *secretproviderfake.Clientset, *certfake.Clientset) {
	return setupTestWithFactory(t, newTestApplicationHandlerFactory(
		config.Config{RequireAppConfigurationItem: requireAppConfigurationItem, RequireAppADGroups: requireAppADGroups},
		func(ctx context.Context, kubeClient kubernetes.Interface, namespace string, configMapName string) (bool, error) {
			return true, nil
		},
	))
}
func setupTestWithFactory(t *testing.T, handlerFactory applications.ApplicationHandlerFactory) (*commontest.Utils, *controllertest.Utils, *kubefake.Clientset, *radixfake.Clientset, *kedafake.Clientset, *prometheusfake.Clientset, *secretproviderfake.Clientset, *certfake.Clientset) {
	// Setup
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
	mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
	mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).AnyTimes().Return(controllertest.NewTestPrincipal(), nil)
	controllerTestUtils := controllertest.NewTestUtils(
		kubeclient,
		radixclient,
		kedaClient,
		secretproviderclient,
		certClient,
		mockValidator,
		applications.NewApplicationController(
			func(_ context.Context, _ kubernetes.Interface, _ v1.RadixRegistration) (bool, error) {
				return true, nil
			},
			handlerFactory,
		),
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

func TestGetApplications_Authenticated(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _, _, _, _, _ := setupTest(t, true, true)
	_, err := commonTestUtils.ApplyRegistration(builders.ARadixRegistration())
	require.NoError(t, err)

	// Test

	mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
	mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).Times(1).Return(controllertest.NewTestPrincipal(), nil)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications"), controllertest.WithValidatorOverride(mockValidator))
	response := <-responseChannel

	applications := make([]applicationModels.ApplicationSummary, 0)
	err = controllertest.GetResponseBody(response, &applications)
	require.NoError(t, err)
	assert.Equal(t, 1, len(applications))
}

func TestGetApplications_Unauthenticated(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _, _, _, _, _ := setupTest(t, true, true)
	_, err := commonTestUtils.ApplyRegistration(builders.ARadixRegistration())
	require.NoError(t, err)

	// Test

	mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
	mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).Times(1).Return(token.NewAnonymousPrincipal(), nil)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications"), controllertest.WithValidatorOverride(mockValidator))
	response := <-responseChannel

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestGetApplications_InvalidToken(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _, _, _, _, _ := setupTest(t, true, true)
	_, err := commonTestUtils.ApplyRegistration(builders.ARadixRegistration())
	require.NoError(t, err)

	// Test

	mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
	mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).Times(1).Return(token.NewAnonymousPrincipal(), radixhttp.ForbiddenError("invalid token"))

	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications"), controllertest.WithValidatorOverride(mockValidator))
	response := <-responseChannel

	assert.Equal(t, http.StatusForbidden, response.Code)
}
