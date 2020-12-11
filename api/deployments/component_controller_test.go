package deployments

import (
	"fmt"
	"testing"

	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	controllertest "github.com/equinor/radix-api/api/test"
	"github.com/equinor/radix-api/api/utils"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
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
	assert.Equal(t, controllertest.AppNotFoundErrorMsg(anyAppName), errorResponse.Message)
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

func TestGetComponents_WithExternalAlias_ContainsTLSSecrets(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, client, radixclient := setupTest()
	utils.ApplyDeploymentWithSync(client, radixclient, commonTestUtils,
		builders.ARadixDeployment().
			WithAppName("any-app").
			WithEnvironment("prod").
			WithDeploymentName(anyDeployName).
			WithComponents(
				builders.NewDeployComponentBuilder().
					WithName("frontend").
					WithPort("http", 8080).
					WithPublicPort("http").
					WithDNSExternalAlias("some.alias.com").
					WithDNSExternalAlias("another.alias.com")))

	// Test
	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	controllertest.GetResponseBody(response, &components)

	assert.Equal(t, 4, len(components[0].Secrets))
	assert.Equal(t, "some.alias.com-cert", components[0].Secrets[0])
	assert.Equal(t, "some.alias.com-key", components[0].Secrets[1])
	assert.Equal(t, "another.alias.com-cert", components[0].Secrets[2])
	assert.Equal(t, "another.alias.com-key", components[0].Secrets[3])
}

func TestGetComponents_WithVolumeMount_ContainsVolumeMountSecrets(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, client, radixclient := setupTest()
	utils.ApplyDeploymentWithSync(client, radixclient, commonTestUtils,
		builders.ARadixDeployment().
			WithAppName("any-app").
			WithEnvironment("prod").
			WithDeploymentName(anyDeployName).
			WithComponents(
				builders.NewDeployComponentBuilder().
					WithName("frontend").
					WithPort("http", 8080).
					WithPublicPort("http").
					WithVolumeMounts([]v1.RadixVolumeMount{
						{
							Type:      v1.MountTypeBlob,
							Name:      "somevolumename",
							Container: "some-container",
							Path:      "some-path",
						},
					})))

	// Test
	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	controllertest.GetResponseBody(response, &components)

	secrets := components[0].Secrets
	assert.Equal(t, 2, len(secrets))
	assert.Contains(t, secrets, "frontend-somevolumename-blobfusecreds-accountkey")
	assert.Contains(t, secrets, "frontend-somevolumename-blobfusecreds-accountname")
}

func TestGetComponents_WithTwoVolumeMounts_ContainsTwoVolumeMountSecrets(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, client, radixclient := setupTest()
	utils.ApplyDeploymentWithSync(client, radixclient, commonTestUtils,
		builders.ARadixDeployment().
			WithAppName("any-app").
			WithEnvironment("prod").
			WithDeploymentName(anyDeployName).
			WithComponents(
				builders.NewDeployComponentBuilder().
					WithName("frontend").
					WithPort("http", 8080).
					WithPublicPort("http").
					WithVolumeMounts([]v1.RadixVolumeMount{
						{
							Type:      v1.MountTypeBlob,
							Name:      "somevolumename1",
							Container: "some-container1",
							Path:      "some-path1",
						},
						{
							Type:      v1.MountTypeBlob,
							Name:      "somevolumename2",
							Container: "some-container2",
							Path:      "some-path2",
						},
					})))

	// Test
	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	controllertest.GetResponseBody(response, &components)

	secrets := components[0].Secrets
	assert.Equal(t, 4, len(secrets))
	assert.Contains(t, secrets, "frontend-somevolumename1-blobfusecreds-accountkey")
	assert.Contains(t, secrets, "frontend-somevolumename1-blobfusecreds-accountname")
	assert.Contains(t, secrets, "frontend-somevolumename2-blobfusecreds-accountkey")
	assert.Contains(t, secrets, "frontend-somevolumename2-blobfusecreds-accountname")
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
		WithCreated(initialDeploymentCreated).
		WithCondition(v1.DeploymentInactive).
		WithActiveFrom(initialDeploymentCreated).
		WithActiveTo(activeDeploymentCreated))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment("dev").
		WithDeploymentName("active-deployment").
		WithCreated(activeDeploymentCreated).
		WithCondition(v1.DeploymentActive).
		WithActiveFrom(activeDeploymentCreated))

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
	assert.Nil(t, components[0].HorizontalScalingSummary)
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
	createComponentPodWithContainerState(kubeclient, builders.GetEnvironmentNamespace(anyAppName, "dev"), message, deploymentModels.Failing, true)

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
	createComponentPodWithContainerState(kubeclient, builders.GetEnvironmentNamespace(anyAppName, "dev"), message, deploymentModels.Running, true)

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

func TestGetComponents_ReplicaStatus_Starting(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _ := setupTest()
	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment("dev").
		WithDeploymentName(anyDeployName))

	message := ""
	createComponentPodWithContainerState(kubeclient, builders.GetEnvironmentNamespace(anyAppName, "dev"), message, deploymentModels.Running, false)

	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	controllertest.GetResponseBody(response, &components)

	assert.Equal(t, 1, len(components))
	assert.Equal(t, 1, len(components[0].ReplicaList))
	assert.Equal(t, deploymentModels.Starting.String(), components[0].ReplicaList[0].Status.Status)
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
	createComponentPodWithContainerState(kubeclient, builders.GetEnvironmentNamespace(anyAppName, "dev"), message, deploymentModels.Pending, true)

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

func TestGetComponents_WithHorizontalScaling(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, client, radixclient := setupTest()
	minReplicas := int32(2)
	maxReplicas := int32(6)
	utils.ApplyDeploymentWithSync(client, radixclient, commonTestUtils,
		builders.ARadixDeployment().
			WithAppName("any-app").
			WithEnvironment("prod").
			WithDeploymentName(anyDeployName).
			WithComponents(
				builders.NewDeployComponentBuilder().
					WithName("frontend").
					WithPort("http", 8080).
					WithPublicPort("http").
					WithHorizontalScaling(&minReplicas, maxReplicas)))

	// Test
	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	controllertest.GetResponseBody(response, &components)

	assert.NotNil(t, components[0].HorizontalScalingSummary)
	assert.Equal(t, minReplicas, components[0].HorizontalScalingSummary.MinReplicas)
	assert.Equal(t, maxReplicas, components[0].HorizontalScalingSummary.MaxReplicas)
	assert.Equal(t, int32(0), components[0].HorizontalScalingSummary.CurrentCPUUtilizationPercentage)
	assert.Equal(t, int32(80), components[0].HorizontalScalingSummary.TargetCPUUtilizationPercentage)
}

func createComponentPodWithContainerState(kubeclient kubernetes.Interface, namespace, message string, status deploymentModels.ContainerStatus, ready bool) {
	podSpec := getPodSpec()
	containerState := getContainerState(message, status)
	podStatus := corev1.PodStatus{
		ContainerStatuses: []corev1.ContainerStatus{
			{
				State: containerState,
				Ready: ready,
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
