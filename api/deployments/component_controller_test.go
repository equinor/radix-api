package deployments

import (
	"context"
	"fmt"
	"strings"
	"testing"

	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	"github.com/equinor/radix-api/api/secrets/suffix"
	controllertest "github.com/equinor/radix-api/api/test"
	"github.com/equinor/radix-api/api/utils"
	radixhttp "github.com/equinor/radix-common/net/http"
	radixutils "github.com/equinor/radix-common/utils"
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
	_, controllerTestUtils, _, _, _, _ := setupTest()

	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 404, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	assert.Equal(t, controllertest.AppNotFoundErrorMsg(anyAppName), errorResponse.Message)
}

func TestGetComponents_non_existing_deployment(t *testing.T) {
	commonTestUtils, controllerTestUtils, _, _, _, _ := setupTest()
	commonTestUtils.ApplyApplication(builders.
		ARadixApplication().
		WithAppName(anyAppName))

	endpoint := createGetComponentsEndpoint(anyAppName, "any-non-existing-deployment")

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 404, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	expectedError := deploymentModels.NonExistingDeployment(nil, "any-non-existing-deployment")

	assert.Equal(t, (expectedError.(*radixhttp.Error)).Message, errorResponse.Message)
}

func TestGetComponents_active_deployment(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _, _, _ := setupTest()
	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithJobComponents(
			builders.NewDeployJobComponentBuilder().WithName("job")).
		WithComponents(
			builders.NewDeployComponentBuilder().WithName("app")).
		WithAppName(anyAppName).
		WithEnvironment("dev").
		WithDeploymentName(anyDeployName))

	createComponentPod(kubeclient, "pod1", builders.GetEnvironmentNamespace(anyAppName, "dev"), "app")
	createComponentPod(kubeclient, "pod2", builders.GetEnvironmentNamespace(anyAppName, "dev"), "app")
	createComponentPod(kubeclient, "pod3", builders.GetEnvironmentNamespace(anyAppName, "dev"), "job")

	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	controllertest.GetResponseBody(response, &components)

	assert.Equal(t, 2, len(components))
	app := getComponentByName("app", components)
	assert.Equal(t, 2, len(app.Replicas))
	job := getComponentByName("job", components)
	assert.Equal(t, 1, len(job.Replicas))
}

func TestGetComponents_WithExternalAlias_ContainsTLSSecrets(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, client, radixclient, promclient, secretProviderClient := setupTest()
	utils.ApplyDeploymentWithSync(client, radixclient, promclient, commonTestUtils, secretProviderClient, builders.ARadixDeployment().
		WithAppName("any-app").
		WithEnvironment("prod").
		WithDeploymentName(anyDeployName).
		WithJobComponents().
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

	frontend := getComponentByName("frontend", components)
	assert.Equal(t, 4, len(frontend.Secrets))
	assert.Equal(t, "some.alias.com-cert", frontend.Secrets[0])
	assert.Equal(t, "some.alias.com-key", frontend.Secrets[1])
	assert.Equal(t, "another.alias.com-cert", frontend.Secrets[2])
	assert.Equal(t, "another.alias.com-key", frontend.Secrets[3])
}

func TestGetComponents_WithVolumeMount_ContainsVolumeMountSecrets(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, client, radixclient, promclient, secretProviderClient := setupTest()
	utils.ApplyDeploymentWithSync(client, radixclient, promclient, commonTestUtils, secretProviderClient, builders.ARadixDeployment().
		WithAppName("any-app").
		WithEnvironment("prod").
		WithDeploymentName(anyDeployName).
		WithJobComponents(
			builders.NewDeployJobComponentBuilder().
				WithName("job").
				WithVolumeMounts(
					v1.RadixVolumeMount{
						Type:      v1.MountTypeBlob,
						Name:      "jobvol",
						Container: "jobcont",
						Path:      "jobpath",
					},
				),
		).
		WithComponents(
			builders.NewDeployComponentBuilder().
				WithName("frontend").
				WithPort("http", 8080).
				WithPublicPort("http").
				WithVolumeMounts(
					v1.RadixVolumeMount{
						Type:      v1.MountTypeBlob,
						Name:      "somevolumename",
						Container: "some-container",
						Path:      "some-path",
					},
				)))

	// Test
	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	controllertest.GetResponseBody(response, &components)

	frontend := getComponentByName("frontend", components)
	secrets := frontend.Secrets
	assert.Equal(t, 2, len(secrets))
	assert.Contains(t, secrets, "frontend-somevolumename-blobfusecreds-accountkey")
	assert.Contains(t, secrets, "frontend-somevolumename-blobfusecreds-accountname")

	job := getComponentByName("job", components)
	secrets = job.Secrets
	assert.Equal(t, 2, len(secrets))
	assert.Contains(t, secrets, "job-jobvol-blobfusecreds-accountkey")
	assert.Contains(t, secrets, "job-jobvol-blobfusecreds-accountname")
}

func TestGetComponents_WithTwoVolumeMounts_ContainsTwoVolumeMountSecrets(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, client, radixclient, promclient, secretProviderClient := setupTest()
	utils.ApplyDeploymentWithSync(client, radixclient, promclient, commonTestUtils, secretProviderClient, builders.ARadixDeployment().
		WithAppName("any-app").
		WithEnvironment("prod").
		WithDeploymentName(anyDeployName).
		WithJobComponents().
		WithComponents(
			builders.NewDeployComponentBuilder().
				WithName("frontend").
				WithPort("http", 8080).
				WithPublicPort("http").
				WithVolumeMounts(
					v1.RadixVolumeMount{
						Type:      v1.MountTypeBlob,
						Name:      "somevolumename1",
						Container: "some-container1",
						Path:      "some-path1",
					},
					v1.RadixVolumeMount{
						Type:      v1.MountTypeBlob,
						Name:      "somevolumename2",
						Container: "some-container2",
						Path:      "some-path2",
					},
				)))

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

func TestGetComponents_OAuth2(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, client, radixclient, promclient, secretProviderClient := setupTest()
	utils.ApplyDeploymentWithSync(client, radixclient, promclient, commonTestUtils, secretProviderClient, builders.ARadixDeployment().
		WithAppName("any-app").
		WithEnvironment("prod").
		WithDeploymentName(anyDeployName).
		WithJobComponents().
		WithComponents(
			builders.NewDeployComponentBuilder().WithName("c1").WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{}}),
			builders.NewDeployComponentBuilder().WithName("c2").WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{SessionStoreType: v1.SessionStoreRedis}}),
			builders.NewDeployComponentBuilder().WithName("c3").WithPublicPort("http"),
			builders.NewDeployComponentBuilder().WithName("c4").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{}}),
		))

	// Test
	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	controllertest.GetResponseBody(response, &components)

	actualComponent := getComponentByName("c1", components)
	assert.NotNil(t, actualComponent.AuxiliaryResource.OAuth2)
	assert.ElementsMatch(t, []string{"c1" + suffix.OAuth2ClientSecret, "c1" + suffix.OAuth2CookieSecret}, actualComponent.Secrets)

	actualComponent = getComponentByName("c2", components)
	assert.NotNil(t, actualComponent.AuxiliaryResource.OAuth2)
	assert.ElementsMatch(t, []string{"c2" + suffix.OAuth2ClientSecret, "c2" + suffix.OAuth2CookieSecret, "c2" + suffix.OAuth2RedisPassword}, actualComponent.Secrets)

	actualComponent = getComponentByName("c3", components)
	assert.Nil(t, actualComponent.AuxiliaryResource.OAuth2)
	assert.Empty(t, actualComponent.Secrets)

	actualComponent = getComponentByName("c4", components)
	assert.Nil(t, actualComponent.AuxiliaryResource.OAuth2)
	assert.Empty(t, actualComponent.Secrets)
}

func TestGetComponents_inactive_deployment(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _, _, _ := setupTest()

	initialDeploymentCreated, _ := radixutils.ParseTimestamp("2018-11-12T11:45:26Z")
	activeDeploymentCreated, _ := radixutils.ParseTimestamp("2018-11-14T11:45:26Z")

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment("dev").
		WithDeploymentName("initial-deployment").
		WithComponents(
			builders.NewDeployComponentBuilder().WithName("app"),
		).
		WithJobComponents(
			builders.NewDeployJobComponentBuilder().WithName("job"),
		).
		WithCreated(initialDeploymentCreated).
		WithCondition(v1.DeploymentInactive).
		WithActiveFrom(initialDeploymentCreated).
		WithActiveTo(activeDeploymentCreated))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment("dev").
		WithDeploymentName("active-deployment").
		WithComponents(
			builders.NewDeployComponentBuilder().WithName("app"),
		).
		WithJobComponents(
			builders.NewDeployJobComponentBuilder().WithName("job"),
		).
		WithCreated(activeDeploymentCreated).
		WithCondition(v1.DeploymentActive).
		WithActiveFrom(activeDeploymentCreated))

	createComponentPod(kubeclient, "pod1", builders.GetEnvironmentNamespace(anyAppName, "dev"), "app")
	createComponentPod(kubeclient, "pod2", builders.GetEnvironmentNamespace(anyAppName, "dev"), "job")

	endpoint := createGetComponentsEndpoint(anyAppName, "initial-deployment")

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	controllertest.GetResponseBody(response, &components)

	assert.Equal(t, 2, len(components))
	app := getComponentByName("app", components)
	assert.Equal(t, 0, len(app.Replicas))
	job := getComponentByName("job", components)
	assert.Equal(t, 0, len(job.Replicas))
}

func createComponentPod(kubeclient kubernetes.Interface, podName, namespace, radixComponentLabel string) {
	podSpec := getPodSpec(podName, radixComponentLabel)
	kubeclient.CoreV1().Pods(namespace).Create(context.TODO(), podSpec, metav1.CreateOptions{})
}

func getPodSpec(podName, radixComponentLabel string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
			Labels: map[string]string{
				kube.RadixComponentLabel: radixComponentLabel,
			},
		},
	}
}

func TestGetComponents_success(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _, _, _ := setupTest()
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

	assert.Equal(t, 2, len(components))
	assert.Nil(t, components[0].HorizontalScalingSummary)
	assert.Nil(t, components[1].HorizontalScalingSummary)
}

func TestGetComponents_ReplicaStatus_Failing(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _, _, _ := setupTest()
	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment("dev").
		WithDeploymentName(anyDeployName).
		WithComponents(
			builders.NewDeployComponentBuilder().WithName("app")).
		WithJobComponents(
			builders.NewDeployJobComponentBuilder().WithName("job")))

	message1 := "Couldn't find key TEST_SECRET in Secret radix-demo-hello-nodejs-dev/www"
	createComponentPodWithContainerState(kubeclient, "pod1", builders.GetEnvironmentNamespace(anyAppName, "dev"), "app", message1, deploymentModels.Failing, true)
	createComponentPodWithContainerState(kubeclient, "pod2", builders.GetEnvironmentNamespace(anyAppName, "dev"), "app", message1, deploymentModels.Failing, true)
	message2 := "Couldn't find key TEST_SECRET in Secret radix-demo-hello-nodejs-dev/job"
	createComponentPodWithContainerState(kubeclient, "pod3", builders.GetEnvironmentNamespace(anyAppName, "dev"), "job", message2, deploymentModels.Failing, true)

	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	controllertest.GetResponseBody(response, &components)

	assert.Equal(t, 2, len(components))
	app := getComponentByName("app", components)
	assert.Equal(t, 2, len(app.ReplicaList))
	assert.Equal(t, deploymentModels.Failing.String(), app.ReplicaList[0].Status.Status)
	assert.Equal(t, message1, app.ReplicaList[0].StatusMessage)

	job := getComponentByName("job", components)
	assert.Equal(t, 1, len(job.ReplicaList))
	assert.Equal(t, deploymentModels.Failing.String(), job.ReplicaList[0].Status.Status)
	assert.Equal(t, message2, job.ReplicaList[0].StatusMessage)
}

func TestGetComponents_ReplicaStatus_Running(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _, _, _ := setupTest()
	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment("dev").
		WithDeploymentName(anyDeployName).
		WithComponents(
			builders.NewDeployComponentBuilder().WithName("app")).
		WithJobComponents(
			builders.NewDeployJobComponentBuilder().WithName("job")))

	message := ""
	createComponentPodWithContainerState(kubeclient, "pod1", builders.GetEnvironmentNamespace(anyAppName, "dev"), "app", message, deploymentModels.Running, true)
	createComponentPodWithContainerState(kubeclient, "pod2", builders.GetEnvironmentNamespace(anyAppName, "dev"), "app", message, deploymentModels.Running, true)
	createComponentPodWithContainerState(kubeclient, "pod3", builders.GetEnvironmentNamespace(anyAppName, "dev"), "job", message, deploymentModels.Running, true)

	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	controllertest.GetResponseBody(response, &components)

	assert.Equal(t, 2, len(components))
	app := getComponentByName("app", components)
	assert.Equal(t, 2, len(app.ReplicaList))
	assert.Equal(t, deploymentModels.Running.String(), app.ReplicaList[0].Status.Status)
	assert.Equal(t, message, app.ReplicaList[0].StatusMessage)

	job := getComponentByName("job", components)
	assert.Equal(t, 1, len(job.ReplicaList))
	assert.Equal(t, deploymentModels.Running.String(), job.ReplicaList[0].Status.Status)
	assert.Equal(t, message, job.ReplicaList[0].StatusMessage)
}

func TestGetComponents_ReplicaStatus_Starting(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _, _, _ := setupTest()
	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment("dev").
		WithDeploymentName(anyDeployName).
		WithComponents(
			builders.NewDeployComponentBuilder().WithName("app")).
		WithJobComponents(
			builders.NewDeployJobComponentBuilder().WithName("job")))

	message := ""
	createComponentPodWithContainerState(kubeclient, "pod1", builders.GetEnvironmentNamespace(anyAppName, "dev"), "app", message, deploymentModels.Running, false)
	createComponentPodWithContainerState(kubeclient, "pod2", builders.GetEnvironmentNamespace(anyAppName, "dev"), "app", message, deploymentModels.Running, false)
	createComponentPodWithContainerState(kubeclient, "pod3", builders.GetEnvironmentNamespace(anyAppName, "dev"), "job", message, deploymentModels.Running, false)

	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	controllertest.GetResponseBody(response, &components)

	assert.Equal(t, 2, len(components))
	app := getComponentByName("app", components)
	assert.Equal(t, 2, len(app.ReplicaList))
	assert.Equal(t, deploymentModels.Starting.String(), app.ReplicaList[0].Status.Status)
	assert.Equal(t, message, app.ReplicaList[0].StatusMessage)

	job := getComponentByName("job", components)
	assert.Equal(t, 1, len(job.ReplicaList))
	assert.Equal(t, deploymentModels.Starting.String(), job.ReplicaList[0].Status.Status)
	assert.Equal(t, message, job.ReplicaList[0].StatusMessage)
}

func TestGetComponents_ReplicaStatus_Pending(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _, _, _ := setupTest()
	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment("dev").
		WithDeploymentName(anyDeployName).
		WithComponents(
			builders.NewDeployComponentBuilder().WithName("app")).
		WithJobComponents(
			builders.NewDeployJobComponentBuilder().WithName("job")))

	message := ""
	createComponentPodWithContainerState(kubeclient, "pod1", builders.GetEnvironmentNamespace(anyAppName, "dev"), "app", message, deploymentModels.Pending, true)
	createComponentPodWithContainerState(kubeclient, "pod2", builders.GetEnvironmentNamespace(anyAppName, "dev"), "app", message, deploymentModels.Pending, true)
	createComponentPodWithContainerState(kubeclient, "pod3", builders.GetEnvironmentNamespace(anyAppName, "dev"), "job", message, deploymentModels.Pending, true)

	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	controllertest.GetResponseBody(response, &components)

	assert.Equal(t, 2, len(components))
	app := getComponentByName("app", components)
	assert.Equal(t, 2, len(app.ReplicaList))
	assert.Equal(t, deploymentModels.Pending.String(), app.ReplicaList[0].Status.Status)
	assert.Equal(t, message, app.ReplicaList[0].StatusMessage)

	job := getComponentByName("job", components)
	assert.Equal(t, 1, len(job.ReplicaList))
	assert.Equal(t, deploymentModels.Pending.String(), job.ReplicaList[0].Status.Status)
	assert.Equal(t, message, job.ReplicaList[0].StatusMessage)
}

func TestGetComponents_WithHorizontalScaling(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, client, radixclient, promclient, secretProviderClient := setupTest()
	minReplicas := int32(2)
	maxReplicas := int32(6)
	utils.ApplyDeploymentWithSync(client, radixclient, promclient, commonTestUtils, secretProviderClient, builders.ARadixDeployment().
		WithAppName("any-app").
		WithEnvironment("prod").
		WithDeploymentName(anyDeployName).
		WithJobComponents().
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

func TestGetComponents_WithIdentity(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, client, radixclient, promclient, secretProviderClient := setupTest()

	utils.ApplyDeploymentWithSync(client, radixclient, promclient, commonTestUtils, secretProviderClient, builders.ARadixDeployment().
		WithAppName("any-app").
		WithEnvironment("prod").
		WithDeploymentName(anyDeployName).
		WithJobComponents(
			builders.NewDeployJobComponentBuilder().
				WithName("job1").
				WithIdentity(&v1.Identity{Azure: &v1.AzureIdentity{ClientId: "job-clientid"}}),
			builders.NewDeployJobComponentBuilder().WithName("job2"),
		).
		WithComponents(
			builders.NewDeployComponentBuilder().
				WithName("comp1").
				WithIdentity(&v1.Identity{Azure: &v1.AzureIdentity{ClientId: "comp-clientid"}}),
			builders.NewDeployComponentBuilder().WithName("comp2"),
		))

	// Test
	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	controllertest.GetResponseBody(response, &components)

	assert.Equal(t, &deploymentModels.Identity{Azure: &deploymentModels.AzureIdentity{ClientId: "job-clientid"}}, getComponentByName("job1", components).Identity)
	assert.Nil(t, getComponentByName("job2", components).Identity)
	assert.Equal(t, &deploymentModels.Identity{Azure: &deploymentModels.AzureIdentity{ClientId: "comp-clientid"}}, getComponentByName("comp1", components).Identity)
	assert.Nil(t, getComponentByName("comp2", components).Identity)
}

func createComponentPodWithContainerState(kubeclient kubernetes.Interface, podName, namespace, radixComponentLabel, message string, status deploymentModels.ContainerStatus, ready bool) {
	podSpec := getPodSpec(podName, radixComponentLabel)
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

	kubeclient.CoreV1().Pods(namespace).Create(context.TODO(), podSpec, metav1.CreateOptions{})
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

func getComponentByName(name string, components []deploymentModels.Component) *deploymentModels.Component {
	for _, comp := range components {
		if strings.EqualFold(name, comp.Name) {
			return &comp
		}
	}
	return nil
}
