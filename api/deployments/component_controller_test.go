package deployments

import (
	"fmt"
	"testing"

	deploymentModels "github.com/statoil/radix-api/api/deployments/models"
	controllertest "github.com/statoil/radix-api/api/test"
	"github.com/statoil/radix-api/api/utils"
	commontest "github.com/statoil/radix-operator/pkg/apis/test"
	builders "github.com/statoil/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func createGetComponentsEndpoint(appName, deployName string) string {
	return fmt.Sprintf("/api/v1/applications/%s/deployments/%s/components", appName, deployName)
}

func applyRegistrationAndAppConfig(commonTestUtils *commontest.Utils) {
	commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithName(anyAppName).
		WithCloneURL(anyCloneURL))
	commonTestUtils.ApplyApplication(builders.
		ARadixApplication().
		WithAppName(anyAppName).
		WithComponents(
			builders.
				NewApplicationComponentBuilder().
				WithName("app")).
		WithEnvironment("dev", "master").
		WithEnvironment("prod", ""))
}

func TestGetComponents_non_existing_app(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _ := setupTest()

	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 404, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	expectedError := nonExistingDeployment(nil, anyDeployName)

	assert.Equal(t, (expectedError.(*utils.Error)).Message, errorResponse.Message)
}

func TestGetComponents_non_existing_deployment(t *testing.T) {
	commonTestUtils, controllerTestUtils, _, _ := setupTest()
	applyRegistrationAndAppConfig(commonTestUtils)

	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 404, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	expectedError := nonExistingDeployment(nil, anyDeployName)

	assert.Equal(t, (expectedError.(*utils.Error)).Message, errorResponse.Message)
}

func TestGetComponents_active_deployment(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _ := setupTest()
	applyRegistrationAndAppConfig(commonTestUtils)
	applyDeployment(kubeclient, commonTestUtils, anyDeployName)

	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.ComponentDeployment
	controllertest.GetResponseBody(response, &components)

	assert.Equal(t, 1, len(components))
	assert.Equal(t, 1, len(components[0].Replicas))
}

func TestGetComponents_inactive_deployment(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _ := setupTest()
	applyRegistrationAndAppConfig(commonTestUtils)
	applyDeployment(kubeclient, commonTestUtils, anyDeployName)
	applyDeployment(kubeclient, commonTestUtils, "another_active_deployment")

	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.ComponentDeployment
	controllertest.GetResponseBody(response, &components)

	assert.Equal(t, 1, len(components))
	assert.Equal(t, 0, len(components[0].Replicas))
}

func applyDeployment(kubeclient kubernetes.Interface, commonTestUtils *commontest.Utils, deploymentName string) {
	deployment := builders.ARadixDeployment().WithAppName(anyAppName).WithDeploymentName(deploymentName)
	rd := deployment.BuildRD()
	ns := rd.GetNamespace()
	commonTestUtils.ApplyDeployment(deployment)
	kubeclient.CoreV1().Pods(ns).Create(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "any-pod",
				Labels: map[string]string{
					"radixComponent": rd.Spec.Components[0].Name,
				},
			},
		},
	)
}

func TestGetComponents_success(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _ := setupTest()
	applyRegistrationAndAppConfig(commonTestUtils)

	commonTestUtils.ApplyDeployment(
		builders.ARadixDeployment().WithAppName(anyAppName).WithDeploymentName(anyDeployName))

	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.ComponentDeployment
	controllertest.GetResponseBody(response, &components)

	assert.Equal(t, 1, len(components))
}
