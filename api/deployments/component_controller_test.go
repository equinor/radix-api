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
	"github.com/equinor/radix-api/api/utils/labelselector"
	radixhttp "github.com/equinor/radix-common/net/http"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	"github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func createGetComponentsEndpoint(appName, deployName string) string {
	return fmt.Sprintf("/api/v1/applications/%s/deployments/%s/components", appName, deployName)
}

func TestGetComponents_non_existing_app(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _, _, _ := setupTest(t)

	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 404, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	assert.Equal(t, controllertest.AppNotFoundErrorMsg(anyAppName), errorResponse.Message)
}

func TestGetComponents_non_existing_deployment(t *testing.T) {
	commonTestUtils, controllerTestUtils, _, _, _, _, _, _ := setupTest(t)
	_, err := commonTestUtils.ApplyApplication(operatorUtils.
		ARadixApplication().
		WithAppName(anyAppName))
	require.NoError(t, err)

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
	commonTestUtils, controllerTestUtils, kubeclient, _, _, _, _, _ := setupTest(t)
	_, err := commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorUtils.
			ARadixDeployment().
			WithJobComponents(
				operatorUtils.NewDeployJobComponentBuilder().WithName("job")).
			WithComponents(
				operatorUtils.NewDeployComponentBuilder().WithName("app")).
			WithAppName(anyAppName).
			WithEnvironment("dev").
			WithDeploymentName(anyDeployName))
	require.NoError(t, err)

	err = createComponentPod(kubeclient, "pod1", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "app")
	require.NoError(t, err)
	err = createComponentPod(kubeclient, "pod2", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "app")
	require.NoError(t, err)
	err = createComponentPod(kubeclient, "pod3", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "job")
	require.NoError(t, err)

	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	err = controllertest.GetResponseBody(response, &components)
	require.NoError(t, err)

	assert.Equal(t, 2, len(components))
	app := getComponentByName("app", components)
	assert.Equal(t, 2, len(app.Replicas)) // nolint:staticcheck // SA1019: Ignore linting deprecated fields
	job := getComponentByName("job", components)
	assert.Equal(t, 1, len(job.Replicas)) // nolint:staticcheck // SA1019: Ignore linting deprecated fields
}

func TestGetComponents_WithVolumeMount_ContainsVolumeMountSecrets(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, client, radixclient, kedaClient, promclient, secretProviderClient, certClient := setupTest(t)
	err := utils.ApplyDeploymentWithSync(client, radixclient, kedaClient, promclient, commonTestUtils, secretProviderClient, certClient, operatorUtils.ARadixDeployment().
		WithAppName("any-app").
		WithEnvironment("prod").
		WithDeploymentName(anyDeployName).
		WithJobComponents(
			operatorUtils.NewDeployJobComponentBuilder().
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
			operatorUtils.NewDeployComponentBuilder().
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
	require.NoError(t, err)

	// Test
	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	err = controllertest.GetResponseBody(response, &components)
	require.NoError(t, err)

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
	commonTestUtils, controllerTestUtils, client, radixclient, kedaClient, promclient, secretProviderClient, certClient := setupTest(t)
	err := utils.ApplyDeploymentWithSync(client, radixclient, kedaClient, promclient, commonTestUtils, secretProviderClient, certClient, operatorUtils.ARadixDeployment().
		WithAppName("any-app").
		WithEnvironment("prod").
		WithDeploymentName(anyDeployName).
		WithJobComponents().
		WithComponents(
			operatorUtils.NewDeployComponentBuilder().
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
	require.NoError(t, err)

	// Test
	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	err = controllertest.GetResponseBody(response, &components)
	require.NoError(t, err)

	secrets := components[0].Secrets
	assert.Equal(t, 4, len(secrets))
	assert.Contains(t, secrets, "frontend-somevolumename1-blobfusecreds-accountkey")
	assert.Contains(t, secrets, "frontend-somevolumename1-blobfusecreds-accountname")
	assert.Contains(t, secrets, "frontend-somevolumename2-blobfusecreds-accountkey")
	assert.Contains(t, secrets, "frontend-somevolumename2-blobfusecreds-accountname")
}

func TestGetComponents_OAuth2(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, client, radixclient, kedaClient, promclient, secretProviderClient, certClient := setupTest(t)
	err := utils.ApplyDeploymentWithSync(client, radixclient, kedaClient, promclient, commonTestUtils, secretProviderClient, certClient, operatorUtils.ARadixDeployment().
		WithAppName("any-app").
		WithEnvironment("prod").
		WithDeploymentName(anyDeployName).
		WithJobComponents().
		WithComponents(
			operatorUtils.NewDeployComponentBuilder().WithName("c1").WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{}}),
			operatorUtils.NewDeployComponentBuilder().WithName("c2").WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{SessionStoreType: v1.SessionStoreRedis}}),
			operatorUtils.NewDeployComponentBuilder().WithName("c3").WithPublicPort("http"),
			operatorUtils.NewDeployComponentBuilder().WithName("c4").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{}}),
		))
	require.NoError(t, err)

	// Test
	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	err = controllertest.GetResponseBody(response, &components)
	require.NoError(t, err)

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
	commonTestUtils, controllerTestUtils, kubeclient, _, _, _, _, _ := setupTest(t)

	initialDeploymentCreated, _ := radixutils.ParseTimestamp("2018-11-12T11:45:26Z")
	activeDeploymentCreated, _ := radixutils.ParseTimestamp("2018-11-14T11:45:26Z")

	_, err := commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorUtils.
			ARadixDeployment().
			WithAppName(anyAppName).
			WithEnvironment("dev").
			WithDeploymentName("initial-deployment").
			WithComponents(
				operatorUtils.NewDeployComponentBuilder().WithName("app"),
			).
			WithJobComponents(
				operatorUtils.NewDeployJobComponentBuilder().WithName("job"),
			).
			WithCreated(initialDeploymentCreated).
			WithCondition(v1.DeploymentInactive).
			WithActiveFrom(initialDeploymentCreated).
			WithActiveTo(activeDeploymentCreated))
	require.NoError(t, err)

	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorUtils.
			ARadixDeployment().
			WithAppName(anyAppName).
			WithEnvironment("dev").
			WithDeploymentName("active-deployment").
			WithComponents(
				operatorUtils.NewDeployComponentBuilder().WithName("app"),
			).
			WithJobComponents(
				operatorUtils.NewDeployJobComponentBuilder().WithName("job"),
			).
			WithCreated(activeDeploymentCreated).
			WithCondition(v1.DeploymentActive).
			WithActiveFrom(activeDeploymentCreated))
	require.NoError(t, err)

	err = createComponentPod(kubeclient, "pod1", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "app")
	require.NoError(t, err)
	err = createComponentPod(kubeclient, "pod2", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "job")
	require.NoError(t, err)

	endpoint := createGetComponentsEndpoint(anyAppName, "initial-deployment")

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	err = controllertest.GetResponseBody(response, &components)
	require.NoError(t, err)

	assert.Equal(t, 2, len(components))
	app := getComponentByName("app", components)
	assert.Equal(t, 0, len(app.Replicas)) // nolint:staticcheck // SA1019: Ignore linting deprecated fields
	job := getComponentByName("job", components)
	assert.Equal(t, 0, len(job.Replicas)) // nolint:staticcheck // SA1019: Ignore linting deprecated fields
}

func createComponentPod(kubeclient kubernetes.Interface, podName, namespace, radixAppLabel, radixComponentLabel string) error {
	podSpec := getPodSpec(podName, radixAppLabel, radixComponentLabel)
	_, err := kubeclient.CoreV1().Pods(namespace).Create(context.Background(), podSpec, metav1.CreateOptions{})
	return err
}

func getPodSpec(podName, radixAppLabel, radixComponentLabel string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
			Labels: map[string]string{
				kube.RadixComponentLabel: radixComponentLabel,
				kube.RadixAppLabel:       radixAppLabel,
			},
		},
	}
}

func TestGetComponents_success(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _, _, _, _, _ := setupTest(t)
	_, err := commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorUtils.
			ARadixDeployment().
			WithAppName(anyAppName).
			WithDeploymentName(anyDeployName))
	require.NoError(t, err)

	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	err = controllertest.GetResponseBody(response, &components)
	require.NoError(t, err)

	assert.Equal(t, 2, len(components))
	assert.Nil(t, components[0].HorizontalScalingSummary)
	assert.Nil(t, components[1].HorizontalScalingSummary)
}

func TestGetComponents_ReplicaStatus_Failing(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _, _, _, _, _ := setupTest(t)
	_, err := commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorUtils.
			ARadixDeployment().
			WithAppName(anyAppName).
			WithEnvironment("dev").
			WithDeploymentName(anyDeployName).
			WithComponents(
				operatorUtils.NewDeployComponentBuilder().WithName("app")).
			WithJobComponents(
				operatorUtils.NewDeployJobComponentBuilder().WithName("job")))
	require.NoError(t, err)

	message1 := "Couldn't find key TEST_SECRET in Secret radix-demo-hello-nodejs-dev/www"
	err = createComponentPodWithContainerState(kubeclient, "pod1", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "app", message1, deploymentModels.Failing, true)
	require.NoError(t, err)
	err = createComponentPodWithContainerState(kubeclient, "pod2", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "app", message1, deploymentModels.Failing, true)
	require.NoError(t, err)
	message2 := "Couldn't find key TEST_SECRET in Secret radix-demo-hello-nodejs-dev/job"
	err = createComponentPodWithContainerState(kubeclient, "pod3", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "job", message2, deploymentModels.Failing, true)
	require.NoError(t, err)

	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	err = controllertest.GetResponseBody(response, &components)
	require.NoError(t, err)

	assert.Equal(t, 2, len(components))
	app := getComponentByName("app", components)
	require.Equal(t, 2, len(app.ReplicaList))
	assert.Equal(t, string(v1.PodFailed), app.ReplicaList[0].Status.Status)
	assert.Equal(t, message1, app.ReplicaList[0].StatusMessage)

	job := getComponentByName("job", components)
	require.Equal(t, 1, len(job.ReplicaList))
	assert.Equal(t, string(v1.PodFailed), job.ReplicaList[0].Status.Status)
	assert.Equal(t, message2, job.ReplicaList[0].StatusMessage)
}

func TestGetComponents_ReplicaStatus_Running(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _, _, _, _, _ := setupTest(t)
	_, err := commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorUtils.
			ARadixDeployment().
			WithAppName(anyAppName).
			WithEnvironment("dev").
			WithDeploymentName(anyDeployName).
			WithComponents(
				operatorUtils.NewDeployComponentBuilder().WithName("app")).
			WithJobComponents(
				operatorUtils.NewDeployJobComponentBuilder().WithName("job")))
	require.NoError(t, err)

	message := ""
	err = createComponentPodWithContainerState(kubeclient, "pod1", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "app", message, deploymentModels.Running, true)
	require.NoError(t, err)
	err = createComponentPodWithContainerState(kubeclient, "pod2", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "app", message, deploymentModels.Running, true)
	require.NoError(t, err)
	err = createComponentPodWithContainerState(kubeclient, "pod3", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "job", message, deploymentModels.Running, true)
	require.NoError(t, err)

	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	err = controllertest.GetResponseBody(response, &components)
	require.NoError(t, err)

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
	commonTestUtils, controllerTestUtils, kubeclient, _, _, _, _, _ := setupTest(t)
	_, err := commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorUtils.
			ARadixDeployment().
			WithAppName(anyAppName).
			WithEnvironment("dev").
			WithDeploymentName(anyDeployName).
			WithComponents(
				operatorUtils.NewDeployComponentBuilder().WithName("app")).
			WithJobComponents(
				operatorUtils.NewDeployJobComponentBuilder().WithName("job")))
	require.NoError(t, err)

	message := ""
	err = createComponentPodWithContainerState(kubeclient, "pod1", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "app", message, deploymentModels.Running, false)
	require.NoError(t, err)
	err = createComponentPodWithContainerState(kubeclient, "pod2", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "app", message, deploymentModels.Running, false)
	require.NoError(t, err)
	err = createComponentPodWithContainerState(kubeclient, "pod3", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "job", message, deploymentModels.Running, false)
	require.NoError(t, err)

	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	err = controllertest.GetResponseBody(response, &components)
	require.NoError(t, err)

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
	commonTestUtils, controllerTestUtils, kubeclient, _, _, _, _, _ := setupTest(t)
	_, err := commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorUtils.
			ARadixDeployment().
			WithAppName(anyAppName).
			WithEnvironment("dev").
			WithDeploymentName(anyDeployName).
			WithComponents(
				operatorUtils.NewDeployComponentBuilder().WithName("app")).
			WithJobComponents(
				operatorUtils.NewDeployJobComponentBuilder().WithName("job")))
	require.NoError(t, err)

	message := ""
	err = createComponentPodWithContainerState(kubeclient, "pod1", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "app", message, deploymentModels.Pending, true)
	require.NoError(t, err)
	err = createComponentPodWithContainerState(kubeclient, "pod2", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "app", message, deploymentModels.Pending, true)
	require.NoError(t, err)
	err = createComponentPodWithContainerState(kubeclient, "pod3", operatorUtils.GetEnvironmentNamespace(anyAppName, "dev"), anyAppName, "job", message, deploymentModels.Pending, true)
	require.NoError(t, err)

	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	err = controllertest.GetResponseBody(response, &components)
	require.NoError(t, err)

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

	testScenarios := []struct {
		name                  string
		deploymentName        string
		minReplicas           int32
		maxReplicas           int32
		targetCpu             *int32
		targetMemory          *int32
		targetCron            *int32
		targetAzureServiceBus *int32
	}{
		{"targetCpu and targetMemory are nil", "dep1", 2, 6, nil, nil, nil, nil},
		{"targetCpu is nil, targetMemory is non-nil", "dep2", 2, 6, nil, pointers.Ptr[int32](75), nil, nil},
		{"targetCpu is non-nil, targetMemory is nil", "dep3", 2, 6, pointers.Ptr[int32](60), nil, nil, nil},
		{"targetCpu and targetMemory are non-nil", "dep4", 2, 6, pointers.Ptr[int32](62), pointers.Ptr[int32](79), nil, nil},
		{"Test CRON trigger is found", "dep5", 2, 6, nil, nil, pointers.Ptr[int32](5), nil},
		{"Test Azure trigger is found", "dep6", 2, 6, nil, nil, nil, pointers.Ptr[int32](15)},
	}

	for _, scenario := range testScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			commonTestUtils, controllerTestUtils, client, radixclient, kedaClient, promclient, secretProviderClient, certClient := setupTest(t)
			err := utils.ApplyDeploymentWithSync(client, radixclient, kedaClient, promclient, commonTestUtils, secretProviderClient, certClient, operatorUtils.ARadixDeployment().
				WithAppName(anyAppName).
				WithEnvironment("prod").
				WithDeploymentName(scenario.deploymentName).
				WithJobComponents().
				WithComponents(
					operatorUtils.NewDeployComponentBuilder().
						WithName("frontend").
						WithPort("http", 8080).
						WithPublicPort("http")))
			require.NoError(t, err)

			ns := operatorUtils.GetEnvironmentNamespace(anyAppName, "prod")
			scaler, hpa := createHorizontalScalingObjects("frontend", numbers.Int32Ptr(scenario.minReplicas), scenario.maxReplicas, scenario.targetCpu, scenario.targetMemory, scenario.targetCron, scenario.targetAzureServiceBus)
			_, err = kedaClient.KedaV1alpha1().ScaledObjects(ns).Create(context.Background(), &scaler, metav1.CreateOptions{})
			require.NoError(t, err)
			_, err = client.AutoscalingV2().HorizontalPodAutoscalers(ns).Create(context.Background(), &hpa, metav1.CreateOptions{})
			require.NoError(t, err)

			// Test
			endpoint := createGetComponentsEndpoint(anyAppName, scenario.deploymentName)
			responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
			response := <-responseChannel

			assert.Equal(t, 200, response.Code)

			var components []deploymentModels.Component
			err = controllertest.GetResponseBody(response, &components)
			require.NoError(t, err)
			require.NotNil(t, components[0].HorizontalScalingSummary)

			assert.Equal(t, scenario.minReplicas, *components[0].HorizontalScalingSummary.MinReplicas)
			assert.Equal(t, scenario.maxReplicas, *components[0].HorizontalScalingSummary.MaxReplicas)
			assert.EqualValues(t, 2, components[0].HorizontalScalingSummary.CurrentReplicas)
			assert.EqualValues(t, 4, components[0].HorizontalScalingSummary.DesiredReplicas)
			assert.Nil(t, components[0].HorizontalScalingSummary.CurrentCPUUtilizationPercentage)                            // nolint:staticcheck // SA1019: Ignore linting deprecated fields
			assert.Equal(t, scenario.targetCpu, components[0].HorizontalScalingSummary.TargetCPUUtilizationPercentage)       // nolint:staticcheck // SA1019: Ignore linting deprecated fields
			assert.Nil(t, components[0].HorizontalScalingSummary.CurrentMemoryUtilizationPercentage)                         // nolint:staticcheck // SA1019: Ignore linting deprecated fields
			assert.Equal(t, scenario.targetMemory, components[0].HorizontalScalingSummary.TargetMemoryUtilizationPercentage) // nolint:staticcheck // SA1019: Ignore linting deprecated fields

			memoryTrigger, ok := slice.FindFirst(components[0].HorizontalScalingSummary.Triggers, func(s deploymentModels.HorizontalScalingSummaryTriggerStatus) bool {
				return s.Name == "memory"
			})
			if scenario.targetMemory == nil {
				assert.False(t, ok)
			} else {
				require.True(t, ok)
				assert.Equal(t, fmt.Sprintf("%d", *scenario.targetMemory), memoryTrigger.TargetUtilization)
				assert.Empty(t, memoryTrigger.CurrentUtilization)
				assert.Empty(t, memoryTrigger.Error)
				assert.Equal(t, "memory", memoryTrigger.Type)
			}

			cpuTrigger, ok := slice.FindFirst(components[0].HorizontalScalingSummary.Triggers, func(s deploymentModels.HorizontalScalingSummaryTriggerStatus) bool {
				return s.Name == "cpu"
			})
			if scenario.targetCpu == nil {
				assert.False(t, ok)
			} else {
				require.True(t, ok)
				assert.Equal(t, fmt.Sprintf("%d", *scenario.targetCpu), cpuTrigger.TargetUtilization)
				assert.Empty(t, cpuTrigger.CurrentUtilization)
				assert.Empty(t, cpuTrigger.Error)
				assert.Equal(t, "cpu", cpuTrigger.Type)
			}

			cronTrigger, ok := slice.FindFirst(components[0].HorizontalScalingSummary.Triggers, func(s deploymentModels.HorizontalScalingSummaryTriggerStatus) bool {
				return s.Name == "cron"
			})
			if scenario.targetCron == nil {
				assert.False(t, ok)
			} else {
				require.True(t, ok)
				assert.Equal(t, fmt.Sprintf("%d", *scenario.targetCron), cronTrigger.TargetUtilization)
				assert.Equal(t, fmt.Sprintf("%d", *scenario.targetCron), cronTrigger.CurrentUtilization)
				assert.Empty(t, cronTrigger.Error)
				assert.Equal(t, "cron", cronTrigger.Type)
			}

			azureTrigger, ok := slice.FindFirst(components[0].HorizontalScalingSummary.Triggers, func(s deploymentModels.HorizontalScalingSummaryTriggerStatus) bool {
				return s.Name == "azure-servicebus"
			})
			if scenario.targetAzureServiceBus == nil {
				assert.False(t, ok)
			} else {
				require.True(t, ok)
				assert.Equal(t, fmt.Sprintf("%d", *scenario.targetAzureServiceBus), azureTrigger.TargetUtilization)
				assert.Equal(t, fmt.Sprintf("%d", *scenario.targetAzureServiceBus), azureTrigger.CurrentUtilization)
				assert.Empty(t, azureTrigger.Error)
				assert.Equal(t, "azure-servicebus", azureTrigger.Type)
			}
		})
	}
}

func createHorizontalScalingObjects(name string, minReplicas *int32, maxReplicas int32, targetCpu *int32, targetMemory *int32, targetCron *int32, targetAzureServiceBus *int32) (v1alpha1.ScaledObject, v2.HorizontalPodAutoscaler) {
	var triggers []v1alpha1.ScaleTriggers
	var metrics []v2.MetricSpec
	resourceMetricNames := []string{}
	externalMetricNames := []string{}
	health := map[string]v1alpha1.HealthStatus{}
	metricStatus := []v2.MetricStatus{}

	if targetCpu != nil {
		resourceMetricNames = append(resourceMetricNames, "cpu")
		triggers = append(triggers, v1alpha1.ScaleTriggers{
			Type: "cpu",
			Name: "cpu",
			Metadata: map[string]string{
				"value": fmt.Sprintf("%d", *targetCpu),
			},
			AuthenticationRef: nil,
			MetricType:        "Utilization",
		})
		metrics = append(metrics, v2.MetricSpec{
			Resource: &v2.ResourceMetricSource{
				Name: "cpu",
				Target: v2.MetricTarget{
					Type:               "cpu",
					AverageUtilization: targetCpu,
				},
			},
		})
	}

	if targetMemory != nil {
		resourceMetricNames = append(resourceMetricNames, "memory")
		triggers = append(triggers, v1alpha1.ScaleTriggers{
			Type: "memory",
			Name: "memory",
			Metadata: map[string]string{
				"value": fmt.Sprintf("%d", *targetMemory),
			},
			MetricType: "Utilization",
		})
		metrics = append(metrics, v2.MetricSpec{
			Resource: &v2.ResourceMetricSource{
				Name: "memory",
				Target: v2.MetricTarget{
					Type:               "memory",
					AverageUtilization: targetMemory,
				},
			},
		})
	}

	if targetCron != nil {
		externalMetricName := fmt.Sprintf("s%d-cron-Europe-Oslo-08xx1-5-016xx1-5", len(triggers))
		externalMetricNames = append(externalMetricNames, externalMetricName)
		triggers = append(triggers, v1alpha1.ScaleTriggers{
			Type: "cron",
			Name: "cron",
			Metadata: map[string]string{
				"end":             "0 16 * * 1-5",
				"start":           "0 8 * * 1-5",
				"timezone":        "Europe/Oslo",
				"desiredReplicas": fmt.Sprintf("%d", *targetCron),
			},
		})
		health[externalMetricName] = v1alpha1.HealthStatus{
			NumberOfFailures: pointers.Ptr[int32](0),
			Status:           "Happy",
		}
		metricStatus = append(metricStatus, v2.MetricStatus{
			Type: "External",
			External: &v2.ExternalMetricStatus{
				Current: v2.MetricValueStatus{
					AverageValue: resource.NewQuantity(int64(*targetCron), resource.DecimalSI),
				},
				Metric: v2.MetricIdentifier{
					Name: externalMetricName,
				},
			},
		})
	}

	if targetAzureServiceBus != nil {
		externalMetricName := fmt.Sprintf("s%d-azure-servicebus-orders", len(triggers))
		externalMetricNames = append(externalMetricNames, externalMetricName)
		triggers = append(triggers, v1alpha1.ScaleTriggers{
			Type: "azure-servicebus",
			Name: "azure-servicebus",
			Metadata: map[string]string{
				"messageCount": fmt.Sprintf("%d", *targetAzureServiceBus),
			},
		})
		health[externalMetricName] = v1alpha1.HealthStatus{
			NumberOfFailures: pointers.Ptr[int32](0),
			Status:           "Happy",
		}
		metricStatus = append(metricStatus, v2.MetricStatus{
			Type: "External",
			External: &v2.ExternalMetricStatus{
				Current: v2.MetricValueStatus{
					AverageValue: resource.NewQuantity(int64(*targetAzureServiceBus), resource.DecimalSI),
				},
				Metric: v2.MetricIdentifier{
					Name: externalMetricName,
				},
			},
		})
	}

	scaler := v1alpha1.ScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labelselector.ForComponent(anyAppName, "frontend"),
		},
		Spec: v1alpha1.ScaledObjectSpec{
			MinReplicaCount: minReplicas,
			MaxReplicaCount: &maxReplicas,
			Triggers:        triggers,
		},
		Status: v1alpha1.ScaledObjectStatus{
			HpaName:             fmt.Sprintf("hpa-%s", name),
			Health:              health,
			ResourceMetricNames: resourceMetricNames,
			ExternalMetricNames: externalMetricNames,
		},
	}

	hpa := v2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:   fmt.Sprintf("hpa-%s", name),
			Labels: labelselector.ForComponent(anyAppName, "frontend"),
		},
		Spec: v2.HorizontalPodAutoscalerSpec{
			MinReplicas: minReplicas,
			MaxReplicas: maxReplicas,
			Metrics:     metrics,
		},
		Status: v2.HorizontalPodAutoscalerStatus{
			CurrentMetrics:  metricStatus,
			CurrentReplicas: 2,
			DesiredReplicas: 4,
		},
	}

	return scaler, hpa
}

func TestGetComponents_WithIdentity(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, client, radixclient, kedaClient, promclient, secretProviderClient, certClient := setupTest(t)

	err := utils.ApplyDeploymentWithSync(client, radixclient, kedaClient, promclient, commonTestUtils, secretProviderClient, certClient, operatorUtils.ARadixDeployment().
		WithAppName("any-app").
		WithEnvironment("prod").
		WithDeploymentName(anyDeployName).
		WithJobComponents(
			operatorUtils.NewDeployJobComponentBuilder().
				WithName("job1").
				WithIdentity(&v1.Identity{Azure: &v1.AzureIdentity{ClientId: "job-clientid"}}).
				WithSecretRefs(v1.RadixSecretRefs{AzureKeyVaults: []v1.RadixAzureKeyVault{{Name: "job-key-vault1", Items: []v1.RadixAzureKeyVaultItem{{Name: "secret1"}}}}}).
				WithSecretRefs(v1.RadixSecretRefs{AzureKeyVaults: []v1.RadixAzureKeyVault{{Name: "job-key-vault2", Items: []v1.RadixAzureKeyVaultItem{{Name: "secret2"}}, UseAzureIdentity: pointers.Ptr(false)}}}).
				WithSecretRefs(v1.RadixSecretRefs{AzureKeyVaults: []v1.RadixAzureKeyVault{{Name: "job-key-vault3", Items: []v1.RadixAzureKeyVaultItem{{Name: "secret3"}}, UseAzureIdentity: pointers.Ptr(true)}}}),
			operatorUtils.NewDeployJobComponentBuilder().WithName("job2"),
		).
		WithComponents(
			operatorUtils.NewDeployComponentBuilder().
				WithName("comp1").
				WithIdentity(&v1.Identity{Azure: &v1.AzureIdentity{ClientId: "comp-clientid"}}).
				WithSecretRefs(v1.RadixSecretRefs{AzureKeyVaults: []v1.RadixAzureKeyVault{{Name: "comp-key-vault1", Items: []v1.RadixAzureKeyVaultItem{{Name: "secret1"}}}}}).
				WithSecretRefs(v1.RadixSecretRefs{AzureKeyVaults: []v1.RadixAzureKeyVault{{Name: "comp-key-vault2", Items: []v1.RadixAzureKeyVaultItem{{Name: "secret2"}}, UseAzureIdentity: pointers.Ptr(false)}}}).
				WithSecretRefs(v1.RadixSecretRefs{AzureKeyVaults: []v1.RadixAzureKeyVault{{Name: "comp-key-vault3", Items: []v1.RadixAzureKeyVaultItem{{Name: "secret3"}}, UseAzureIdentity: pointers.Ptr(true)}}}),
			operatorUtils.NewDeployComponentBuilder().WithName("comp2"),
		))
	require.NoError(t, err)

	// Test
	endpoint := createGetComponentsEndpoint(anyAppName, anyDeployName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", endpoint)
	response := <-responseChannel

	assert.Equal(t, 200, response.Code)

	var components []deploymentModels.Component
	err = controllertest.GetResponseBody(response, &components)
	require.NoError(t, err)

	assert.Equal(t, &deploymentModels.Identity{Azure: &deploymentModels.AzureIdentity{ClientId: "job-clientid", ServiceAccountName: operatorUtils.GetComponentServiceAccountName("job1"), AzureKeyVaults: []string{"job-key-vault3"}}}, getComponentByName("job1", components).Identity)
	assert.Nil(t, getComponentByName("job2", components).Identity)
	assert.Equal(t, &deploymentModels.Identity{Azure: &deploymentModels.AzureIdentity{ClientId: "comp-clientid", ServiceAccountName: operatorUtils.GetComponentServiceAccountName("comp1"), AzureKeyVaults: []string{"comp-key-vault3"}}}, getComponentByName("comp1", components).Identity)
	assert.Nil(t, getComponentByName("comp2", components).Identity)
}

func createComponentPodWithContainerState(kubeclient kubernetes.Interface, podName, namespace, radixAppLabel, radixComponentLabel, message string, status deploymentModels.ContainerStatus, ready bool) error {
	podSpec := getPodSpec(podName, radixAppLabel, radixComponentLabel)
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

	_, err := kubeclient.CoreV1().Pods(namespace).Create(context.Background(), podSpec, metav1.CreateOptions{})
	return err
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
