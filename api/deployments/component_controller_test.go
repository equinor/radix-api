package deployments

import (
	"fmt"
	"testing"

	deploymentModels "github.com/statoil/radix-api/api/deployments/models"
	controllertest "github.com/statoil/radix-api/api/test"
	"github.com/statoil/radix-api/api/utils"
	builders "github.com/statoil/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func createGetComponentsEndpoint(appName, deployName string) string {
	return fmt.Sprintf("/api/v1/applications/%s/deployments/%s/components", appName, deployName)
}

func TestGetComponents_non_existing_app(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _ := setupTest()

	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 404, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	expectedError := deploymentModels.NonExistingDeployment(nil, anyDeployName)

	assert.Equal(t, (expectedError.(*utils.Error)).Message, errorResponse.Message)
}

func TestGetComponents_non_existing_deployment(t *testing.T) {
	commonTestUtils, controllerTestUtils, _, _ := setupTest()
	commonTestUtils.ApplyApplication(builders.
		ARadixApplication().
		WithAppName(anyAppName))

	endpoint := createGetComponentsEndpoint(anyAppName, "any-non-existing-deployment")

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 404, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	expectedError := deploymentModels.NonExistingDeployment(nil, "any-non-existing-deployment")

	assert.Equal(t, (expectedError.(*utils.Error)).Message, errorResponse.Message)
}

func TestGetComponents_active_deployment(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _ := setupTest()
	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment("dev").
		WithDeploymentName(anyDeployName))

	createComponentPod(kubeclient, builders.GetEnvironmentNamespace(anyAppName, "dev"))

	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	controllertest.GetResponseBody(response, &components)

	assert.Equal(t, 1, len(components))
	assert.Equal(t, 1, len(components[0].Replicas))
}

func TestGetComponents_inactive_deployment(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _ := setupTest()

	initialDeploymentCreated, _ := utils.ParseTimestamp("2018-11-12T11:45:26-0000")
	activeDeploymentCreated, _ := utils.ParseTimestamp("2018-11-14T11:45:26-0000")

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment("dev").
		WithDeploymentName("initial-deployment").
		WithCreated(initialDeploymentCreated))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment("dev").
		WithDeploymentName("active-deployment").
		WithCreated(activeDeploymentCreated))

	createComponentPod(kubeclient, builders.GetEnvironmentNamespace(anyAppName, "dev"))
	createComponentPod(kubeclient, builders.GetEnvironmentNamespace(anyAppName, "dev"))

	endpoint := createGetComponentsEndpoint(anyAppName, "initial-deployment")

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	controllertest.GetResponseBody(response, &components)

	assert.Equal(t, 1, len(components))
	assert.Equal(t, 0, len(components[0].Replicas))
}

func createComponentPod(kubeclient kubernetes.Interface, namespace string) {
	kubeclient.CoreV1().Pods(namespace).Create(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "any-pod",
				Labels: map[string]string{
					"radixComponent": "app",
				},
			},
		},
	)
}

func TestGetComponents_success(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _ := setupTest()
	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName(anyAppName).
		WithDeploymentName(anyDeployName))

	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	controllertest.GetResponseBody(response, &components)

	assert.Equal(t, 1, len(components))
}
