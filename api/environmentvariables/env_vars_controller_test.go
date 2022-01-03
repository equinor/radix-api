package environmentvariables

import (
	"fmt"
	"testing"

	envvarsmodels "github.com/equinor/radix-api/api/environmentvariables/models"
	controllertest "github.com/equinor/radix-api/api/test"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	builders "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	prometheusclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	secretsstorevclient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

const (
	clusterName       = "AnyClusterName"
	containerRegistry = "any.container.registry"
	appName           = "any-app"
	environmentName   = "dev"
	componentName     = "backend"
	egressIps         = "0.0.0.0"
)

func setupTestWithMockHandler(mockCtrl *gomock.Controller) (*commontest.Utils, *controllertest.Utils, kubernetes.Interface, radixclient.Interface, prometheusclient.Interface, *MockEnvVarsHandler) {
	kubeclient, radixclient, prometheusclient, commonTestUtils, _, _ := setupTest()

	handler := NewMockEnvVarsHandler(mockCtrl)
	handlerFactory := NewMockenvVarsHandlerFactory(mockCtrl)
	handlerFactory.EXPECT().createHandler(gomock.Any()).Return(handler)
	controller := (&envVarsController{}).withHandlerFactory(handlerFactory)
	// controllerTestUtils is used for issuing HTTP request and processing responses
	controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, controller)

	return &commonTestUtils, &controllerTestUtils, kubeclient, radixclient, prometheusclient, handler
}

func setupTest() (*kubefake.Clientset, *fake.Clientset, *prometheusfake.Clientset, commontest.Utils, *kube.Kube, secretsstorevclient.Interface) {
	// Setup
	kubeclient := kubefake.NewSimpleClientset()
	radixclient := fake.NewSimpleClientset()
	prometheusclient := prometheusfake.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()

	// commonTestUtils is used for creating CRDs
	commonTestUtils := commontest.NewTestUtils(kubeclient, radixclient)
	commonTestUtils.CreateClusterPrerequisites(clusterName, containerRegistry, egressIps)
	return kubeclient, radixclient, prometheusclient, commonTestUtils, commonTestUtils.GetKubeUtil(), secretproviderclient
}

func Test_GetComponentEnvVars(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	url := fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/envvars", appName, environmentName, componentName)

	t.Run("Return env-vars", func(t *testing.T) {
		commonTestUtils, controllerTestUtils, _, _, _, handler := setupTestWithMockHandler(mockCtrl)
		setupDeployment(commonTestUtils, appName, environmentName, componentName, nil)
		handler.EXPECT().GetComponentEnvVars(appName, environmentName, componentName).
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
		controllertest.GetResponseBody(response, &envVars)

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

		commonTestUtils, controllerTestUtils, _, _, _, handler := setupTestWithMockHandler(mockCtrl)
		setupDeployment(commonTestUtils, appName, environmentName, componentName, nil)
		handler.EXPECT().GetComponentEnvVars(appName, environmentName, componentName).
			Return(nil, fmt.Errorf("some-err"))

		responseChannel := controllerTestUtils.ExecuteRequest("GET", url)
		response := <-responseChannel

		assert.Equal(t, 400, response.Code)
		errorResponse, _ := controllertest.GetErrorResponse(response)
		assert.NotNil(t, errorResponse)
		assert.Equal(t, "Error: some-err", errorResponse.Message)

		var envVars []envvarsmodels.EnvVar
		controllertest.GetResponseBody(response, &envVars)
		assert.Empty(t, envVars)
	})
}

func Test_ChangeEnvVar(t *testing.T) {
	//setupTestWithMockHandler()
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

		commonTestUtils, controllerTestUtils, _, _, _, handler := setupTestWithMockHandler(mockCtrl)
		setupDeployment(commonTestUtils, appName, environmentName, componentName, nil)

		handler.EXPECT().ChangeEnvVar(appName, environmentName, componentName, envVarsParams).
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

		commonTestUtils, controllerTestUtils, _, _, _, handler := setupTestWithMockHandler(mockCtrl)
		setupDeployment(commonTestUtils, appName, environmentName, componentName, nil)

		handler.EXPECT().ChangeEnvVar(appName, environmentName, componentName, envVarsParams).
			Return(fmt.Errorf("some-err"))

		responseChannel := controllerTestUtils.ExecuteRequestWithParameters("PATCH", url, envVarsParams)
		response := <-responseChannel

		assert.Equal(t, 400, response.Code)
		errorResponse, _ := controllertest.GetErrorResponse(response)
		assert.NotNil(t, errorResponse)
		assert.Equal(t, "Error: some-err", errorResponse.Message)
	})
}

func setupDeployment(commonTestUtils *commontest.Utils, appName, environmentName, componentName string, modifyComponentBuilder func(builders.DeployComponentBuilder)) {
	componentBuilder := builders.NewDeployComponentBuilder().WithName(componentName)
	if modifyComponentBuilder != nil {
		modifyComponentBuilder(componentBuilder)
	}
	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithDeploymentName("some-depl").
		WithAppName(appName).
		WithEnvironment(environmentName).
		WithComponent(componentBuilder).
		WithImageTag("1234"))
}
