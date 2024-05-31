package environmentvariables

import (
	"context"
	"fmt"
	"testing"

	certclientfake "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned/fake"
	envvarsmodels "github.com/equinor/radix-api/api/environmentvariables/models"
	controllertest "github.com/equinor/radix-api/api/test"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	builders "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	prometheusclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

const (
	clusterName     = "AnyClusterName"
	appName         = "any-app"
	environmentName = "dev"
	componentName   = "backend"
	egressIps       = "0.0.0.0"
	subscriptionId  = "12347718-c8f8-4995-bfbb-02655ff1f89c"
)

func setupTestWithMockHandler(t *testing.T, mockCtrl *gomock.Controller) (*commontest.Utils, *controllertest.Utils, kubernetes.Interface, radixclient.Interface, prometheusclient.Interface, *MockEnvVarsHandler) {
	kubeclient, radixclient, kedaClient, prometheusclient, commonTestUtils, _, secretproviderclient, certClient := setupTest(t)

	handler := NewMockEnvVarsHandler(mockCtrl)
	handlerFactory := NewMockenvVarsHandlerFactory(mockCtrl)
	handlerFactory.EXPECT().createHandler(gomock.Any()).Return(handler)
	controller := (&envVarsController{}).withHandlerFactory(handlerFactory)
	// controllerTestUtils is used for issuing HTTP request and processing responses
	controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, kedaClient, secretproviderclient, certClient, controller)

	return &commonTestUtils, &controllerTestUtils, kubeclient, radixclient, prometheusclient, handler
}

func setupTest(t *testing.T) (*kubefake.Clientset, *radixfake.Clientset, *kedafake.Clientset, *prometheusfake.Clientset, commontest.Utils, *kube.Kube, *secretproviderfake.Clientset, *certclientfake.Clientset) {
	// Setup
	kubeclient := kubefake.NewSimpleClientset()
	radixclient := radixfake.NewSimpleClientset()
	kedaClient := kedafake.NewSimpleClientset()
	prometheusclient := prometheusfake.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	certClient := certclientfake.NewSimpleClientset()

	// commonTestUtils is used for creating CRDs
	commonTestUtils := commontest.NewTestUtils(kubeclient, radixclient, kedaClient, secretproviderclient)
	err := commonTestUtils.CreateClusterPrerequisites(clusterName, egressIps, subscriptionId)
	require.NoError(t, err)
	return kubeclient, radixclient, kedaClient, prometheusclient, commonTestUtils, commonTestUtils.GetKubeUtil(), secretproviderclient, certClient
}

func Test_GetComponentEnvVars(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	url := fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/envvars", appName, environmentName, componentName)

	t.Run("Return env-vars", func(t *testing.T) {
		commonTestUtils, controllerTestUtils, _, _, _, handler := setupTestWithMockHandler(t, mockCtrl)
		_, err := setupDeployment(commonTestUtils, appName, environmentName, componentName, nil)
		require.NoError(t, err)
		handler.EXPECT().GetComponentEnvVars(gomock.Any(), appName, environmentName, componentName).
			Return([]envvarsmodels.EnvVar{
				{
					Name:     "VAR1",
					Value:    "val1",
					Metadata: &envvarsmodels.EnvVarMetadata{RadixConfigValue: "orig-val1"},
				},
				{
					Name:     "VAR2",
					Value:    "val2",
					Metadata: nil,
				},
			}, nil)

		responseChannel := controllerTestUtils.ExecuteRequest("GET", url)
		response := <-responseChannel

		assert.Equal(t, 200, response.Code)
		errorResponse, _ := controllertest.GetErrorResponse(response)
		assert.Nil(t, errorResponse)

		var envVars []envvarsmodels.EnvVar
		err = controllertest.GetResponseBody(response, &envVars)
		require.NoError(t, err)

		assert.NotNil(t, envVars)
		assert.NotEmpty(t, envVars)
		assert.Equal(t, "VAR1", envVars[0].Name)
		assert.Equal(t, "val1", envVars[0].Value)
		assert.NotEmpty(t, envVars[0].Metadata)
		assert.Equal(t, "orig-val1", envVars[0].Metadata.RadixConfigValue)
		assert.Equal(t, "VAR2", envVars[1].Name)
		assert.Equal(t, "val2", envVars[1].Value)
		assert.Nil(t, envVars[1].Metadata)
	})

	t.Run("Return error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		commonTestUtils, controllerTestUtils, _, _, _, handler := setupTestWithMockHandler(t, mockCtrl)
		_, err := setupDeployment(commonTestUtils, appName, environmentName, componentName, nil)
		require.NoError(t, err)
		handler.EXPECT().GetComponentEnvVars(gomock.Any(), appName, environmentName, componentName).
			Return(nil, fmt.Errorf("some-err"))

		responseChannel := controllerTestUtils.ExecuteRequest("GET", url)
		response := <-responseChannel

		assert.Equal(t, 400, response.Code)
		errorResponse, _ := controllertest.GetErrorResponse(response)
		assert.NotNil(t, errorResponse)
		assert.Equal(t, "Error: some-err", errorResponse.Message)

		var envVars []envvarsmodels.EnvVar
		_ = controllertest.GetResponseBody(response, &envVars)
		assert.Empty(t, envVars)
	})
}

func Test_ChangeEnvVar(t *testing.T) {
	// setupTestWithMockHandler(t, )
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	url := fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/envvars", appName, environmentName, componentName)
	envVarsParams := []envvarsmodels.EnvVarParameter{
		{
			Name:  "VAR1",
			Value: "val1",
		},
		{
			Name:  "VAR2",
			Value: "val2",
		},
	}

	t.Run("Successfully changed env-vars", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		commonTestUtils, controllerTestUtils, _, _, _, handler := setupTestWithMockHandler(t, mockCtrl)
		_, err := setupDeployment(commonTestUtils, appName, environmentName, componentName, nil)
		require.NoError(t, err)

		handler.EXPECT().ChangeEnvVar(gomock.Any(), appName, environmentName, componentName, envVarsParams).
			Return(nil)

		responseChannel := controllerTestUtils.ExecuteRequestWithParameters("PATCH", url, envVarsParams)
		response := <-responseChannel

		assert.Equal(t, 200, response.Code)
		errorResponse, _ := controllertest.GetErrorResponse(response)
		assert.Nil(t, errorResponse)
	})
	t.Run("Return error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		commonTestUtils, controllerTestUtils, _, _, _, handler := setupTestWithMockHandler(t, mockCtrl)
		_, err := setupDeployment(commonTestUtils, appName, environmentName, componentName, nil)
		require.NoError(t, err)

		handler.EXPECT().ChangeEnvVar(gomock.Any(), appName, environmentName, componentName, envVarsParams).
			Return(fmt.Errorf("some-err"))

		responseChannel := controllerTestUtils.ExecuteRequestWithParameters("PATCH", url, envVarsParams)
		response := <-responseChannel

		assert.Equal(t, 400, response.Code)
		errorResponse, _ := controllertest.GetErrorResponse(response)
		assert.NotNil(t, errorResponse)
		assert.Equal(t, "Error: some-err", errorResponse.Message)
	})
}

func setupDeployment(commonTestUtils *commontest.Utils, appName, environmentName, componentName string, modifyComponentBuilder func(builders.DeployComponentBuilder)) (*radixv1.RadixDeployment, error) {
	componentBuilder := builders.NewDeployComponentBuilder().WithName(componentName)
	if modifyComponentBuilder != nil {
		modifyComponentBuilder(componentBuilder)
	}
	return commonTestUtils.ApplyDeployment(
		context.Background(),
		builders.
			ARadixDeployment().
			WithDeploymentName("some-depl").
			WithAppName(appName).
			WithEnvironment(environmentName).
			WithComponent(componentBuilder).
			WithImageTag("1234"))
}
