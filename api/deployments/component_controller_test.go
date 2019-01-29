package deployments

import (
	"fmt"
	"testing"

	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	controllertest "github.com/equinor/radix-api/api/test"
	"github.com/equinor/radix-api/api/utils"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	builders "github.com/equinor/radix-operator/pkg/apis/utils"
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
	expectedError := deploymentModels.NonExistingRegistration(nil, anyAppName)

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

	initialDeploymentCreated, _ := utils.ParseTimestamp("2018-11-12T11:45:26Z")
	activeDeploymentCreated, _ := utils.ParseTimestamp("2018-11-14T11:45:26Z")

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
	podSpec := getPodSpec()
	kubeclient.CoreV1().Pods(namespace).Create(podSpec)
}

func getPodSpec() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "any-pod",
			Labels: map[string]string{
				kube.RadixComponentLabel: "app",
			},
		},
	}
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

func TestGetComponents_ReplicaStatus_Failing(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _ := setupTest()
	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment("dev").
		WithDeploymentName(anyDeployName))

	message := "Couldn't find key TEST_SECRET in Secret radix-demo-hello-nodejs-dev/www"
	createComponentPodWithContainerState(kubeclient, builders.GetEnvironmentNamespace(anyAppName, "dev"), message, deploymentModels.Failing)

	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	controllertest.GetResponseBody(response, &components)

	assert.Equal(t, 1, len(components))
	assert.Equal(t, 1, len(components[0].ReplicaList))
	assert.Equal(t, deploymentModels.Failing.String(), components[0].ReplicaList[0].Status.Status)
	assert.Equal(t, message, components[0].ReplicaList[0].StatusMessage)
}

func TestGetComponents_ReplicaStatus_Running(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _ := setupTest()
	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment("dev").
		WithDeploymentName(anyDeployName))

	message := ""
	createComponentPodWithContainerState(kubeclient, builders.GetEnvironmentNamespace(anyAppName, "dev"), message, deploymentModels.Running)

	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	controllertest.GetResponseBody(response, &components)

	assert.Equal(t, 1, len(components))
	assert.Equal(t, 1, len(components[0].ReplicaList))
	assert.Equal(t, deploymentModels.Running.String(), components[0].ReplicaList[0].Status.Status)
	assert.Equal(t, message, components[0].ReplicaList[0].StatusMessage)
}

func TestGetComponents_ReplicaStatus_Pending(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _ := setupTest()
	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment("dev").
		WithDeploymentName(anyDeployName))

	message := ""
	createComponentPodWithContainerState(kubeclient, builders.GetEnvironmentNamespace(anyAppName, "dev"), message, deploymentModels.Pending)

	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	controllertest.GetResponseBody(response, &components)

	assert.Equal(t, 1, len(components))
	assert.Equal(t, 1, len(components[0].Replicas))
	assert.Equal(t, deploymentModels.Pending.String(), components[0].ReplicaList[0].Status.Status)
	assert.Equal(t, message, components[0].ReplicaList[0].StatusMessage)
}

func createComponentPodWithContainerState(kubeclient kubernetes.Interface, namespace, message string, status deploymentModels.ContainerStatus) {
	podSpec := getPodSpec()
	containerState := getContainerState(message, status)
	podStatus := corev1.PodStatus{
		ContainerStatuses: []corev1.ContainerStatus{
			{
				State: containerState,
			},
		},
	}
	podSpec.Status = podStatus

	kubeclient.CoreV1().Pods(namespace).Create(podSpec)
}

func getContainerState(message string, status deploymentModels.ContainerStatus) corev1.ContainerState {
	var containerState corev1.ContainerState

	if status == deploymentModels.Failing {
		containerState = corev1.ContainerState{
			Waiting: &corev1.ContainerStateWaiting{
				Message: message,
				Reason:  "",
			},
		}
	}
	if status == deploymentModels.Pending {
		containerState = corev1.ContainerState{
			Waiting: &corev1.ContainerStateWaiting{
				Message: message,
				Reason:  "ContainerCreating",
			},
		}
	}
	if status == deploymentModels.Running {
		containerState = corev1.ContainerState{
			Running: &corev1.ContainerStateRunning{},
		}
	}
	if status == deploymentModels.Terminated {
		containerState = corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				Message: message,
			},
		}
	}

	return containerState
}
