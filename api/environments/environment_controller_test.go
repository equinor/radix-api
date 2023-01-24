package environments

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/equinor/radix-api/api/secrets"
	secretModels "github.com/equinor/radix-api/api/secrets/models"
	"github.com/equinor/radix-api/api/secrets/suffix"
	"github.com/equinor/radix-api/api/utils"
	secretsstorevclient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"

	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	environmentModels "github.com/equinor/radix-api/api/environments/models"
	event "github.com/equinor/radix-api/api/events"
	eventMock "github.com/equinor/radix-api/api/events/mock"
	eventModels "github.com/equinor/radix-api/api/events/models"
	controllertest "github.com/equinor/radix-api/api/test"
	"github.com/equinor/radix-api/models"
	radixmodels "github.com/equinor/radix-common/models"
	radixhttp "github.com/equinor/radix-common/net/http"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/numbers"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	prometheusclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

const (
	clusterName       = "AnyClusterName"
	containerRegistry = "any.container.registry"
	anyAppName        = "any-app"
	anyComponentName  = "app"
	anyJobName        = "job"
	anyEnvironment    = "dev"
	anySecretName     = "TEST_SECRET"
	egressIps         = "0.0.0.0"
)

func setupTest() (*commontest.Utils, *controllertest.Utils, *controllertest.Utils, kubernetes.Interface, radixclient.Interface, prometheusclient.Interface, secretsstorevclient.Interface) {
	// Setup
	kubeclient := kubefake.NewSimpleClientset()
	radixclient := fake.NewSimpleClientset()
	prometheusclient := prometheusfake.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()

	// commonTestUtils is used for creating CRDs
	commonTestUtils := commontest.NewTestUtils(kubeclient, radixclient, secretproviderclient)
	commonTestUtils.CreateClusterPrerequisites(clusterName, containerRegistry, egressIps)

	// secretControllerTestUtils is used for issuing HTTP request and processing responses
	secretControllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, secretproviderclient, secrets.NewSecretController())
	// controllerTestUtils is used for issuing HTTP request and processing responses
	environmentControllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, secretproviderclient, NewEnvironmentController())

	return &commonTestUtils, &environmentControllerTestUtils, &secretControllerTestUtils, kubeclient, radixclient, prometheusclient, secretproviderclient
}

func TestGetEnvironmentDeployments_SortedWithFromTo(t *testing.T) {
	deploymentOneImage := "abcdef"
	deploymentTwoImage := "ghijkl"
	deploymentThreeImage := "mnopqr"
	layout := "2006-01-02T15:04:05.000Z"
	deploymentOneCreated, _ := time.Parse(layout, "2018-11-12T11:45:26.371Z")
	deploymentTwoCreated, _ := time.Parse(layout, "2018-11-12T12:30:14.000Z")
	deploymentThreeCreated, _ := time.Parse(layout, "2018-11-20T09:00:00.000Z")

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _ := setupTest()
	setupGetDeploymentsTest(commonTestUtils, anyAppName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated, anyEnvironment)

	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/deployments", anyAppName, anyEnvironment))
	response := <-responseChannel

	deployments := make([]*deploymentModels.DeploymentSummary, 0)
	controllertest.GetResponseBody(response, &deployments)
	assert.Equal(t, 3, len(deployments))

	assert.Equal(t, deploymentThreeImage, deployments[0].Name)
	assert.Equal(t, radixutils.FormatTimestamp(deploymentThreeCreated), deployments[0].ActiveFrom)
	assert.Equal(t, "", deployments[0].ActiveTo)

	assert.Equal(t, deploymentTwoImage, deployments[1].Name)
	assert.Equal(t, radixutils.FormatTimestamp(deploymentTwoCreated), deployments[1].ActiveFrom)
	assert.Equal(t, radixutils.FormatTimestamp(deploymentThreeCreated), deployments[1].ActiveTo)

	assert.Equal(t, deploymentOneImage, deployments[2].Name)
	assert.Equal(t, radixutils.FormatTimestamp(deploymentOneCreated), deployments[2].ActiveFrom)
	assert.Equal(t, radixutils.FormatTimestamp(deploymentTwoCreated), deployments[2].ActiveTo)
}

func TestGetEnvironmentDeployments_Latest(t *testing.T) {
	deploymentOneImage := "abcdef"
	deploymentTwoImage := "ghijkl"
	deploymentThreeImage := "mnopqr"
	layout := "2006-01-02T15:04:05.000Z"
	deploymentOneCreated, _ := time.Parse(layout, "2018-11-12T11:45:26.371Z")
	deploymentTwoCreated, _ := time.Parse(layout, "2018-11-12T12:30:14.000Z")
	deploymentThreeCreated, _ := time.Parse(layout, "2018-11-20T09:00:00.000Z")

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _ := setupTest()
	setupGetDeploymentsTest(commonTestUtils, anyAppName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated, anyEnvironment)

	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/deployments?latest=true", anyAppName, anyEnvironment))
	response := <-responseChannel

	deployments := make([]*deploymentModels.DeploymentSummary, 0)
	controllertest.GetResponseBody(response, &deployments)
	assert.Equal(t, 1, len(deployments))

	assert.Equal(t, deploymentThreeImage, deployments[0].Name)
	assert.Equal(t, radixutils.FormatTimestamp(deploymentThreeCreated), deployments[0].ActiveFrom)
	assert.Equal(t, "", deployments[0].ActiveTo)
}

func TestGetEnvironmentSummary_ApplicationWithNoDeployments_EnvironmentPending(t *testing.T) {
	envName1, envName2 := "dev", "master"

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _ := setupTest()
	commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithRadixRegistration(operatorutils.ARadixRegistration()).
		WithAppName(anyAppName).
		WithEnvironment(envName1, envName2))

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments", anyAppName))
	response := <-responseChannel
	environments := make([]*environmentModels.EnvironmentSummary, 0)
	controllertest.GetResponseBody(response, &environments)
	assert.Equal(t, 1, len(environments))

	assert.Equal(t, envName1, environments[0].Name)
	assert.Equal(t, environmentModels.Pending.String(), environments[0].Status)
	assert.Equal(t, envName2, environments[0].BranchMapping)
	assert.Nil(t, environments[0].ActiveDeployment)
}

func TestGetEnvironmentSummary_ApplicationWithDeployment_EnvironmentConsistent(t *testing.T) {
	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _ := setupTest()
	commonTestUtils.ApplyDeployment(operatorutils.
		ARadixDeployment().
		WithRadixApplication(operatorutils.
			NewRadixApplicationBuilder().
			WithRadixRegistration(operatorutils.ARadixRegistration()).
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment, "master")).
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment))

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments", anyAppName))
	response := <-responseChannel
	environments := make([]*environmentModels.EnvironmentSummary, 0)
	controllertest.GetResponseBody(response, &environments)

	assert.Equal(t, environmentModels.Consistent.String(), environments[0].Status)
	assert.NotNil(t, environments[0].ActiveDeployment)
}

func TestGetEnvironmentSummary_RemoveEnvironmentFromConfig_OrphanedEnvironment(t *testing.T) {
	envName1, envName2 := "dev", "master"
	anyOrphanedEnvironment := "feature-1"

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _ := setupTest()
	commonTestUtils.ApplyRegistration(operatorutils.
		NewRegistrationBuilder().
		WithName(anyAppName))
	commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment(envName1, envName2).
		WithEnvironment(anyOrphanedEnvironment, "feature"))
	commonTestUtils.ApplyDeployment(operatorutils.
		NewDeploymentBuilder().
		WithAppName(anyAppName).
		WithEnvironment(envName1).
		WithImageTag("someimageindev"))
	commonTestUtils.ApplyDeployment(operatorutils.
		NewDeploymentBuilder().
		WithAppName(anyAppName).
		WithEnvironment(anyOrphanedEnvironment).
		WithImageTag("someimageinfeature"))

	// Remove feature environment from application config
	commonTestUtils.ApplyApplicationUpdate(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment(envName1, envName2))

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments", anyAppName))
	response := <-responseChannel
	environments := make([]*environmentModels.EnvironmentSummary, 0)
	controllertest.GetResponseBody(response, &environments)

	for _, environment := range environments {
		if strings.EqualFold(environment.Name, anyOrphanedEnvironment) {
			assert.Equal(t, environmentModels.Orphan.String(), environment.Status)
			assert.NotNil(t, environment.ActiveDeployment)
		}
	}
}

func TestGetEnvironmentSummary_OrphanedEnvironmentWithDash_OrphanedEnvironmentIsListedOk(t *testing.T) {
	anyOrphanedEnvironment := "feature-1"

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _ := setupTest()
	rr, _ := commonTestUtils.ApplyRegistration(operatorutils.
		NewRegistrationBuilder().
		WithName(anyAppName))
	commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment, "master"))
	commonTestUtils.ApplyEnvironment(operatorutils.
		NewEnvironmentBuilder().
		WithAppLabel().
		WithAppName(anyAppName).
		WithEnvironmentName(anyOrphanedEnvironment).
		WithRegistrationOwner(rr).
		WithOrphaned(true))

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments", anyAppName))
	response := <-responseChannel
	environments := make([]*environmentModels.EnvironmentSummary, 0)
	controllertest.GetResponseBody(response, &environments)

	environmentListed := false
	for _, environment := range environments {
		if strings.EqualFold(environment.Name, anyOrphanedEnvironment) {
			assert.Equal(t, environmentModels.Orphan.String(), environment.Status)
			environmentListed = true
		}
	}

	assert.True(t, environmentListed)
}

func TestDeleteEnvironment_OneOrphanedEnvironment_OnlyOrphanedCanBeDeleted(t *testing.T) {
	anyNonOrphanedEnvironment := "dev-1"
	anyOrphanedEnvironment := "feature-1"

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _ := setupTest()
	commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment(anyNonOrphanedEnvironment, "master").
		WithRadixRegistration(operatorutils.
			NewRegistrationBuilder().
			WithName(anyAppName)))
	commonTestUtils.ApplyEnvironment(operatorutils.
		NewEnvironmentBuilder().
		WithAppLabel().
		WithAppName(anyAppName).
		WithEnvironmentName(anyOrphanedEnvironment))

	// Test
	// Start with two environments
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments", anyAppName))
	response := <-responseChannel
	environments := make([]*environmentModels.EnvironmentSummary, 0)
	controllertest.GetResponseBody(response, &environments)
	assert.Equal(t, 2, len(environments))

	// Orphaned environment can be deleted
	responseChannel = environmentControllerTestUtils.ExecuteRequest("DELETE", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyOrphanedEnvironment))
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)

	// Non-orphaned cannot
	responseChannel = environmentControllerTestUtils.ExecuteRequest("DELETE", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyNonOrphanedEnvironment))
	response = <-responseChannel
	assert.Equal(t, http.StatusBadRequest, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	expectedError := environmentModels.CannotDeleteNonOrphanedEnvironment(anyAppName, anyNonOrphanedEnvironment)
	assert.Equal(t, (expectedError.(*radixhttp.Error)).Message, errorResponse.Message)

	// Only one remaining environment after delete
	responseChannel = environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments", anyAppName))
	response = <-responseChannel
	environments = make([]*environmentModels.EnvironmentSummary, 0)
	controllertest.GetResponseBody(response, &environments)
	assert.Equal(t, 1, len(environments))
}

func TestGetEnvironment_NoExistingEnvironment_ReturnsAnError(t *testing.T) {
	anyNonExistingEnvironment := "non-existing-environment"

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _ := setupTest()
	commonTestUtils.ApplyApplication(operatorutils.
		ARadixApplication().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment, "master"))

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyNonExistingEnvironment))
	response := <-responseChannel
	assert.Equal(t, http.StatusNotFound, response.Code)

	errorResponse, _ := controllertest.GetErrorResponse(response)
	expectedError := environmentModels.NonExistingEnvironment(nil, anyAppName, anyNonExistingEnvironment)
	assert.Equal(t, (expectedError.(*radixhttp.Error)).Message, errorResponse.Message)
}

func TestGetEnvironment_ExistingEnvironmentInConfig_ReturnsAPendingEnvironment(t *testing.T) {
	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _ := setupTest()
	commonTestUtils.ApplyApplication(operatorutils.
		ARadixApplication().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment, "master"))

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyEnvironment))
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)

	environment := environmentModels.Environment{}
	err := controllertest.GetResponseBody(response, &environment)
	assert.Nil(t, err)
	assert.Equal(t, anyEnvironment, environment.Name)
	assert.Equal(t, environmentModels.Pending.String(), environment.Status)
}

func setupGetDeploymentsTest(commonTestUtils *commontest.Utils, appName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage string, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated time.Time, environment string) {
	commonTestUtils.ApplyDeployment(operatorutils.
		ARadixDeployment().
		WithDeploymentName(deploymentOneImage).
		WithAppName(appName).
		WithEnvironment(environment).
		WithImageTag(deploymentOneImage).
		WithCreated(deploymentOneCreated).
		WithCondition(v1.DeploymentInactive).
		WithActiveFrom(deploymentOneCreated).
		WithActiveTo(deploymentTwoCreated))

	commonTestUtils.ApplyDeployment(operatorutils.
		ARadixDeployment().
		WithDeploymentName(deploymentTwoImage).
		WithAppName(appName).
		WithEnvironment(environment).
		WithImageTag(deploymentTwoImage).
		WithCreated(deploymentTwoCreated).
		WithCondition(v1.DeploymentInactive).
		WithActiveFrom(deploymentTwoCreated).
		WithActiveTo(deploymentThreeCreated))

	commonTestUtils.ApplyDeployment(operatorutils.
		ARadixDeployment().
		WithDeploymentName(deploymentThreeImage).
		WithAppName(appName).
		WithEnvironment(environment).
		WithImageTag(deploymentThreeImage).
		WithCreated(deploymentThreeCreated).
		WithCondition(v1.DeploymentActive).
		WithActiveFrom(deploymentThreeCreated))
}

func TestRestartComponent_ApplicationWithDeployment_EnvironmentConsistent(t *testing.T) {
	zeroReplicas := 0
	appName, envName := anyAppName, anyEnvironment
	stoppedComponent, startedComponent := "stoppedComponent", "startedComponent"

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, client, radixclient, _, _ := setupTest()
	rd, _ := createRadixDeploymentWithReplicas(commonTestUtils, appName, envName, []ComponentCreatorStruct{
		{name: stoppedComponent, number: 0},
		{name: startedComponent, number: 1},
	})

	t.Run("Component Restart Succeeds", func(t *testing.T) {
		component := findComponentInDeployment(rd, startedComponent)
		assert.True(t, *component.Replicas > zeroReplicas)

		// Emulate a started component
		createComponentPod(client, rd.GetNamespace(), startedComponent)

		responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/restart", appName, envName, startedComponent))
		response := <-responseChannel
		assert.Equal(t, http.StatusOK, response.Code)

		updatedRd, _ := radixclient.RadixV1().RadixDeployments(rd.GetNamespace()).Get(context.TODO(), rd.GetName(), metav1.GetOptions{})
		component = findComponentInDeployment(updatedRd, startedComponent)
		assert.True(t, *component.Replicas > zeroReplicas)
		assert.NotEmpty(t, component.EnvironmentVariables[defaults.RadixRestartEnvironmentVariable])
	})

	t.Run("Component Restart Fails", func(t *testing.T) {
		component := findComponentInDeployment(rd, stoppedComponent)
		assert.True(t, *component.Replicas == zeroReplicas)

		// Emulate a stopped component
		deleteComponentPod(client, rd.GetNamespace(), stoppedComponent)

		responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/start", appName, envName, stoppedComponent))
		response := <-responseChannel
		assert.Equal(t, http.StatusOK, response.Code)

		updatedRd, _ := radixclient.RadixV1().RadixDeployments(rd.GetNamespace()).Get(context.TODO(), rd.GetName(), metav1.GetOptions{})
		component = findComponentInDeployment(updatedRd, stoppedComponent)
		assert.True(t, *component.Replicas > zeroReplicas)

		responseChannel = environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/restart", appName, envName, stoppedComponent))
		response = <-responseChannel
		// Since pods are not appearing out of nowhere with kubernetes-fake, the component will be in
		// a reconciling state and cannot be restarted
		assert.Equal(t, http.StatusBadRequest, response.Code)

		errorResponse, _ := controllertest.GetErrorResponse(response)
		expectedError := environmentModels.CannotRestartComponent(appName, stoppedComponent, deploymentModels.ComponentReconciling.String())
		assert.Equal(t, (expectedError.(*radixhttp.Error)).Message, errorResponse.Message)
	})
}

func TestStartComponent_ApplicationWithDeployment_EnvironmentConsistent(t *testing.T) {
	zeroReplicas := 0
	appName, envName := anyAppName, anyEnvironment
	stoppedComponent1, stoppedComponent2 := "stoppedComponent1", "stoppedComponent2"

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, client, radixclient, _, _ := setupTest()
	rd, _ := createRadixDeploymentWithReplicas(commonTestUtils, appName, envName, []ComponentCreatorStruct{
		{name: stoppedComponent1, number: 0},
		{name: stoppedComponent2, number: 0},
	})

	t.Run("Component Start Succeeds", func(t *testing.T) {
		component := findComponentInDeployment(rd, stoppedComponent1)
		assert.True(t, *component.Replicas == zeroReplicas)

		responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/start", appName, envName, stoppedComponent1))
		response := <-responseChannel
		assert.Equal(t, http.StatusOK, response.Code)

		updatedRd, _ := radixclient.RadixV1().RadixDeployments(rd.GetNamespace()).Get(context.Background(), rd.GetName(), metav1.GetOptions{})
		component = findComponentInDeployment(updatedRd, stoppedComponent1)
		assert.True(t, *component.Replicas > zeroReplicas)
	})

	t.Run("Component Start Fails", func(t *testing.T) {
		component := findComponentInDeployment(rd, stoppedComponent2)
		assert.True(t, *component.Replicas == zeroReplicas)

		// Create pod
		createComponentPod(client, rd.GetNamespace(), stoppedComponent2)

		responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/start", appName, envName, stoppedComponent2))
		response := <-responseChannel
		// Since pods are not appearing out of nowhere with kubernetes-fake, the component will be in
		// a reconciling state and cannot be started
		assert.Equal(t, http.StatusBadRequest, response.Code)

		errorResponse, _ := controllertest.GetErrorResponse(response)
		expectedError := environmentModels.CannotStartComponent(appName, stoppedComponent2, deploymentModels.ComponentReconciling.String())
		assert.Equal(t, (expectedError.(*radixhttp.Error)).Message, errorResponse.Message)

		updatedRd, _ := radixclient.RadixV1().RadixDeployments(rd.GetNamespace()).Get(context.Background(), rd.GetName(), metav1.GetOptions{})
		component = findComponentInDeployment(updatedRd, stoppedComponent2)
		assert.True(t, *component.Replicas == zeroReplicas)
	})
}

func TestStopComponent_ApplicationWithDeployment_EnvironmentConsistent(t *testing.T) {
	zeroReplicas := 0
	appName, envName := anyAppName, anyEnvironment
	runningComponent, stoppedComponent := "runningComp", "stoppedComponent"

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, radixclient, _, _ := setupTest()
	rd, _ := createRadixDeploymentWithReplicas(commonTestUtils, appName, envName, []ComponentCreatorStruct{
		{name: runningComponent, number: 3},
		{name: stoppedComponent, number: 0},
	})

	// Test
	t.Run("Stop Component Succeeds", func(t *testing.T) {
		component := findComponentInDeployment(rd, runningComponent)
		assert.True(t, *component.Replicas > zeroReplicas)

		responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/stop", appName, envName, runningComponent))
		response := <-responseChannel
		// Since pods are not appearing out of nowhere with kubernetes-fake, the component will be in
		// a reconciling state because number of replicas in spec > 0. Therefore it can be stopped
		assert.Equal(t, http.StatusOK, response.Code)

		updatedRd, _ := radixclient.RadixV1().RadixDeployments(rd.GetNamespace()).Get(context.Background(), rd.GetName(), metav1.GetOptions{})
		component = findComponentInDeployment(updatedRd, runningComponent)
		assert.True(t, *component.Replicas == zeroReplicas)
	})

	t.Run("Stop Component Fails", func(t *testing.T) {
		component := findComponentInDeployment(rd, stoppedComponent)
		assert.True(t, *component.Replicas == zeroReplicas)

		responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/stop", appName, envName, stoppedComponent))
		response := <-responseChannel
		// The component is in a stopped state since replicas in spec = 0, and therefore cannot be stopped again
		assert.Equal(t, http.StatusBadRequest, response.Code)

		errorResponse, _ := controllertest.GetErrorResponse(response)
		expectedError := environmentModels.CannotStopComponent(appName, stoppedComponent, deploymentModels.StoppedComponent.String())
		assert.Equal(t, (expectedError.(*radixhttp.Error)).Message, errorResponse.Message)

		updatedRd, _ := radixclient.RadixV1().RadixDeployments(rd.GetNamespace()).Get(context.Background(), rd.GetName(), metav1.GetOptions{})
		component = findComponentInDeployment(updatedRd, stoppedComponent)
		assert.True(t, *component.Replicas == zeroReplicas)
	})
}

func TestStopEnvrionment_ApplicationWithDeployment_EnvironmentConsistent(t *testing.T) {
	zeroReplicas := 0
	appName := anyAppName

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, radixclient, _, _ := setupTest()

	// Test
	t.Run("Stop Environment", func(t *testing.T) {
		envName := "fullyRunningEnv"
		rd, _ := createRadixDeploymentWithReplicas(commonTestUtils, appName, envName, []ComponentCreatorStruct{
			{name: "runningComponent1", number: 3},
			{name: "runningComponent2", number: 7},
		})
		for _, comp := range rd.Spec.Components {
			assert.True(t, *comp.Replicas > zeroReplicas)
		}

		responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/stop", appName, envName))
		response := <-responseChannel
		// Since pods are not appearing out of nowhere with kubernetes-fake, the component will be in
		// a reconciling state because number of replicas in spec > 0. Therefore it can be stopped
		assert.Equal(t, http.StatusOK, response.Code)

		updatedRd, _ := radixclient.RadixV1().RadixDeployments(rd.GetNamespace()).Get(context.Background(), rd.GetName(), metav1.GetOptions{})
		for _, comp := range updatedRd.Spec.Components {
			assert.True(t, *comp.Replicas == zeroReplicas)
		}
	})

	t.Run("Stop Environment with stopped component", func(t *testing.T) {
		envName := "partiallyRunningEnv"
		rd, _ := createRadixDeploymentWithReplicas(commonTestUtils, appName, envName, []ComponentCreatorStruct{
			{name: "stoppedComponent", number: 0},
			{name: "runningComponent", number: 7},
		})
		replicaCount := 0
		for _, comp := range rd.Spec.Components {
			replicaCount += *comp.Replicas
		}
		assert.True(t, replicaCount > zeroReplicas)

		responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/stop", appName, envName))
		response := <-responseChannel
		// The component is in a stopped state since replicas in spec = 0, and therefore cannot be stopped again
		assert.Equal(t, http.StatusOK, response.Code)

		errorResponse, _ := controllertest.GetErrorResponse(response)
		assert.Nil(t, errorResponse)

		updatedRd, _ := radixclient.RadixV1().RadixDeployments(rd.GetNamespace()).Get(context.Background(), rd.GetName(), metav1.GetOptions{})
		for _, comp := range updatedRd.Spec.Components {
			assert.True(t, *comp.Replicas == zeroReplicas)
		}
	})
}

func TestCreateEnvironment(t *testing.T) {
	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _ := setupTest()
	commonTestUtils.ApplyApplication(operatorutils.
		ARadixApplication().
		WithAppName(anyAppName))

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyEnvironment))
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
}

func Test_GetEnvironmentEvents_Controller(t *testing.T) {
	envName := "dev"

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, kubeClient, _, _, _ := setupTest()
	createEvent := func(namespace, eventName string) {
		kubeClient.CoreV1().Events(namespace).CreateWithEventNamespace(&corev1.Event{
			ObjectMeta: metav1.ObjectMeta{
				Name: eventName,
			},
		})
	}
	createEvent(operatorutils.GetEnvironmentNamespace(anyAppName, envName), "ev1")
	createEvent(operatorutils.GetEnvironmentNamespace(anyAppName, envName), "ev2")
	commonTestUtils.ApplyApplication(operatorutils.
		ARadixApplication().
		WithAppName(anyAppName).
		WithEnvironment(envName, "master"))

	t.Run("Get events for dev environment", func(t *testing.T) {
		responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/events", anyAppName, envName))
		response := <-responseChannel
		assert.Equal(t, http.StatusOK, response.Code)
		events := make([]eventModels.Event, 0)
		controllertest.GetResponseBody(response, &events)
		assert.Len(t, events, 2)
	})

	t.Run("Get events for non-existing environment", func(t *testing.T) {
		responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/events", anyAppName, "prod"))
		response := <-responseChannel
		assert.Equal(t, http.StatusNotFound, response.Code)
		errResponse, _ := controllertest.GetErrorResponse(response)
		assert.Equal(
			t,
			environmentModels.NonExistingEnvironment(nil, anyAppName, "prod").Error(),
			errResponse.Message,
		)
	})

	t.Run("Get events for non-existing application", func(t *testing.T) {
		responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/events", "noapp", envName))
		response := <-responseChannel
		assert.Equal(t, http.StatusNotFound, response.Code)
		errResponse, _ := controllertest.GetErrorResponse(response)
		assert.Equal(
			t,
			controllertest.AppNotFoundErrorMsg("noapp"),
			errResponse.Message,
		)
	})
}

// secret tests
func TestUpdateSecret_TLSSecretForExternalAlias_UpdatedOk(t *testing.T) {
	// Setup
	commonTestUtils, environmentControllerTestUtils, controllerTestUtils, client, radixclient, promclient, secretproviderclient := setupTest()
	utils.ApplyDeploymentWithSync(client, radixclient, promclient, commonTestUtils, secretproviderclient, operatorutils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithRadixApplication(operatorutils.ARadixApplication().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment, "master").
			WithDNSExternalAlias("some.alias.com", anyEnvironment, anyComponentName).
			WithDNSExternalAlias("another.alias.com", anyEnvironment, anyComponentName)).
		WithComponents(
			operatorutils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSExternalAlias("some.alias.com").
				WithDNSExternalAlias("another.alias.com")))

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyEnvironment))
	response := <-responseChannel

	environment := environmentModels.Environment{}
	controllertest.GetResponseBody(response, &environment)
	assert.Equal(t, 4, len(environment.Secrets))
	assert.True(t, contains(environment.Secrets, "some.alias.com-cert"))
	assert.True(t, contains(environment.Secrets, "some.alias.com-key"))
	assert.True(t, contains(environment.Secrets, "another.alias.com-cert"))
	assert.True(t, contains(environment.Secrets, "another.alias.com-key"))

	parameters := secretModels.SecretParameters{
		SecretValue: "anyValue",
	}

	putUrl := fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/secrets/%s", anyAppName, anyEnvironment, anyComponentName, environment.Secrets[0].Name)
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", putUrl, parameters)
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/secrets/%s", anyAppName, anyEnvironment, anyComponentName, environment.Secrets[1].Name), parameters)
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
}

func TestUpdateSecret_AccountSecretForComponentVolumeMount_UpdatedOk(t *testing.T) {
	// Setup
	commonTestUtils, environmentControllerTestUtils, controllerTestUtils, client, radixclient, promclient, secretProviderClient := setupTest()
	utils.ApplyDeploymentWithSync(client, radixclient, promclient, commonTestUtils, secretProviderClient, operatorutils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithRadixApplication(operatorutils.ARadixApplication().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment, "master")).
		WithComponents(
			operatorutils.NewDeployComponentBuilder().
				WithName(anyComponentName).
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
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyEnvironment))
	response := <-responseChannel

	environment := environmentModels.Environment{}
	controllertest.GetResponseBody(response, &environment)
	assert.Equal(t, 2, len(environment.Secrets))
	assert.True(t, contains(environment.Secrets, fmt.Sprintf("%v-somevolumename-blobfusecreds-accountkey", anyComponentName)))
	assert.True(t, contains(environment.Secrets, fmt.Sprintf("%v-somevolumename-blobfusecreds-accountname", anyComponentName)))

	parameters := secretModels.SecretParameters{SecretValue: "anyValue"}
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/secrets/%s", anyAppName, anyEnvironment, anyComponentName, environment.Secrets[0].Name), parameters)
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
}

func TestUpdateSecret_AccountSecretForJobVolumeMount_UpdatedOk(t *testing.T) {
	// Setup
	commonTestUtils, environmentControllerTestUtils, controllerTestUtils, client, radixclient, promclient, secretProviderClient := setupTest()
	utils.ApplyDeploymentWithSync(client, radixclient, promclient, commonTestUtils, secretProviderClient, operatorutils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithRadixApplication(operatorutils.ARadixApplication().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment, "master")).
		WithJobComponents(
			operatorutils.NewDeployJobComponentBuilder().
				WithName(anyJobName).
				WithVolumeMounts(
					v1.RadixVolumeMount{
						Type:      v1.MountTypeBlob,
						Name:      "somevolumename",
						Container: "some-container",
						Path:      "some-path",
					},
				)))

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyEnvironment))
	response := <-responseChannel

	environment := environmentModels.Environment{}
	controllertest.GetResponseBody(response, &environment)
	assert.Equal(t, 2, len(environment.Secrets))
	assert.True(t, contains(environment.Secrets, fmt.Sprintf("%v-somevolumename-blobfusecreds-accountkey", anyJobName)))
	assert.True(t, contains(environment.Secrets, fmt.Sprintf("%v-somevolumename-blobfusecreds-accountname", anyJobName)))

	parameters := secretModels.SecretParameters{SecretValue: "anyValue"}
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/secrets/%s", anyAppName, anyEnvironment, anyJobName, environment.Secrets[0].Name), parameters)
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
}

func TestUpdateSecret_OAuth2_UpdatedOk(t *testing.T) {
	// Setup
	envNs := operatorutils.GetEnvironmentNamespace(anyAppName, anyEnvironment)
	commonTestUtils, environmentControllerTestUtils, controllerTestUtils, client, radixclient, promclient, secretProviderClient := setupTest()
	utils.ApplyDeploymentWithSync(client, radixclient, promclient, commonTestUtils, secretProviderClient, operatorutils.NewDeploymentBuilder().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithRadixApplication(operatorutils.ARadixApplication().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment, "master")).
		WithComponents(
			operatorutils.NewDeployComponentBuilder().WithName(anyComponentName).WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{SessionStoreType: v1.SessionStoreRedis}}),
		),
	)

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyEnvironment))
	response := <-responseChannel

	environment := environmentModels.Environment{}
	controllertest.GetResponseBody(response, &environment)
	assert.ElementsMatch(t, []string{anyComponentName + suffix.OAuth2ClientSecret, anyComponentName + suffix.OAuth2CookieSecret, anyComponentName + suffix.OAuth2RedisPassword}, environment.ActiveDeployment.Components[0].Secrets)

	// Update secret when k8s secret object is missing should return 404
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters(
		"PUT",
		fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/secrets/%s", anyAppName, anyEnvironment, anyComponentName, anyComponentName+suffix.OAuth2ClientSecret),
		secretModels.SecretParameters{SecretValue: "clientsecret"},
	)
	response = <-responseChannel
	assert.Equal(t, http.StatusNotFound, response.Code)

	// Update client secret when k8s secret exists should set Data
	secretName := operatorutils.GetAuxiliaryComponentSecretName(anyComponentName, defaults.OAuthProxyAuxiliaryComponentSuffix)
	client.CoreV1().Secrets(envNs).Create(context.Background(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName}}, metav1.CreateOptions{})

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters(
		"PUT",
		fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/secrets/%s", anyAppName, anyEnvironment, anyComponentName, anyComponentName+suffix.OAuth2ClientSecret),
		secretModels.SecretParameters{SecretValue: "clientsecret"},
	)
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
	actualSecret, _ := client.CoreV1().Secrets(envNs).Get(context.Background(), secretName, metav1.GetOptions{})
	assert.Equal(t, actualSecret.Data, map[string][]byte{defaults.OAuthClientSecretKeyName: []byte("clientsecret")})

	// Update client secret when k8s secret exists should set Data
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters(
		"PUT",
		fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/secrets/%s", anyAppName, anyEnvironment, anyComponentName, anyComponentName+suffix.OAuth2CookieSecret),
		secretModels.SecretParameters{SecretValue: "cookiesecret"},
	)
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
	actualSecret, _ = client.CoreV1().Secrets(envNs).Get(context.Background(), secretName, metav1.GetOptions{})
	assert.Equal(t, actualSecret.Data, map[string][]byte{defaults.OAuthClientSecretKeyName: []byte("clientsecret"), defaults.OAuthCookieSecretKeyName: []byte("cookiesecret")})

	// Update client secret when k8s secret exists should set Data
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters(
		"PUT",
		fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/secrets/%s", anyAppName, anyEnvironment, anyComponentName, anyComponentName+suffix.OAuth2RedisPassword),
		secretModels.SecretParameters{SecretValue: "redispassword"},
	)
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
	actualSecret, _ = client.CoreV1().Secrets(envNs).Get(context.Background(), secretName, metav1.GetOptions{})
	assert.Equal(t, actualSecret.Data, map[string][]byte{defaults.OAuthClientSecretKeyName: []byte("clientsecret"), defaults.OAuthCookieSecretKeyName: []byte("cookiesecret"), defaults.OAuthRedisPasswordKeyName: []byte("redispassword")})
}

func TestGetSecretDeployments_SortedWithFromTo(t *testing.T) {
	deploymentOneImage := "abcdef"
	deploymentTwoImage := "ghijkl"
	deploymentThreeImage := "mnopqr"
	layout := "2006-01-02T15:04:05.000Z"
	deploymentOneCreated, _ := time.Parse(layout, "2018-11-12T11:45:26.371Z")
	deploymentTwoCreated, _ := time.Parse(layout, "2018-11-12T12:30:14.000Z")
	deploymentThreeCreated, _ := time.Parse(layout, "2018-11-20T09:00:00.000Z")

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _ := setupTest()
	setupGetDeploymentsTest(commonTestUtils, anyAppName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated, anyEnvironment)

	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/deployments", anyAppName, anyEnvironment))
	response := <-responseChannel

	deployments := make([]*deploymentModels.DeploymentSummary, 0)
	controllertest.GetResponseBody(response, &deployments)
	assert.Equal(t, 3, len(deployments))

	assert.Equal(t, deploymentThreeImage, deployments[0].Name)
	assert.Equal(t, radixutils.FormatTimestamp(deploymentThreeCreated), deployments[0].ActiveFrom)
	assert.Equal(t, "", deployments[0].ActiveTo)

	assert.Equal(t, deploymentTwoImage, deployments[1].Name)
	assert.Equal(t, radixutils.FormatTimestamp(deploymentTwoCreated), deployments[1].ActiveFrom)
	assert.Equal(t, radixutils.FormatTimestamp(deploymentThreeCreated), deployments[1].ActiveTo)

	assert.Equal(t, deploymentOneImage, deployments[2].Name)
	assert.Equal(t, radixutils.FormatTimestamp(deploymentOneCreated), deployments[2].ActiveFrom)
	assert.Equal(t, radixutils.FormatTimestamp(deploymentTwoCreated), deployments[2].ActiveTo)
}

func TestGetSecretDeployments_Latest(t *testing.T) {
	deploymentOneImage := "abcdef"
	deploymentTwoImage := "ghijkl"
	deploymentThreeImage := "mnopqr"
	layout := "2006-01-02T15:04:05.000Z"
	deploymentOneCreated, _ := time.Parse(layout, "2018-11-12T11:45:26.371Z")
	deploymentTwoCreated, _ := time.Parse(layout, "2018-11-12T12:30:14.000Z")
	deploymentThreeCreated, _ := time.Parse(layout, "2018-11-20T09:00:00.000Z")

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _ := setupTest()
	setupGetDeploymentsTest(commonTestUtils, anyAppName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated, anyEnvironment)

	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/deployments?latest=true", anyAppName, anyEnvironment))
	response := <-responseChannel

	deployments := make([]*deploymentModels.DeploymentSummary, 0)
	controllertest.GetResponseBody(response, &deployments)
	assert.Equal(t, 1, len(deployments))

	assert.Equal(t, deploymentThreeImage, deployments[0].Name)
	assert.Equal(t, radixutils.FormatTimestamp(deploymentThreeCreated), deployments[0].ActiveFrom)
	assert.Equal(t, "", deployments[0].ActiveTo)
}

func TestGetEnvironmentSummary_ApplicationWithNoDeployments_SecretPending(t *testing.T) {
	envName1, envName2 := "dev", "master"

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _ := setupTest()
	commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithRadixRegistration(operatorutils.ARadixRegistration()).
		WithAppName(anyAppName).
		WithEnvironment(envName1, envName2))

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments", anyAppName))
	response := <-responseChannel
	environments := make([]*environmentModels.EnvironmentSummary, 0)
	controllertest.GetResponseBody(response, &environments)

	assert.Equal(t, 1, len(environments))
	assert.Equal(t, envName1, environments[0].Name)
	assert.Equal(t, environmentModels.Pending.String(), environments[0].Status)
	assert.Equal(t, envName2, environments[0].BranchMapping)
	assert.Nil(t, environments[0].ActiveDeployment)
}

func TestGetEnvironmentSummary_ApplicationWithDeployment_SecretConsistent(t *testing.T) {
	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _ := setupTest()
	commonTestUtils.ApplyDeployment(operatorutils.
		ARadixDeployment().
		WithRadixApplication(operatorutils.
			NewRadixApplicationBuilder().
			WithRadixRegistration(operatorutils.ARadixRegistration()).
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment, "master")).
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment))

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments", anyAppName))
	response := <-responseChannel
	environments := make([]*environmentModels.EnvironmentSummary, 0)
	controllertest.GetResponseBody(response, &environments)

	assert.Equal(t, environmentModels.Consistent.String(), environments[0].Status)
	assert.NotNil(t, environments[0].ActiveDeployment)
}

func TestGetEnvironmentSummary_RemoveSecretFromConfig_OrphanedSecret(t *testing.T) {
	envName1, envName2 := "dev", "master"
	anyOrphanedSecret := "feature-1"

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _ := setupTest()
	commonTestUtils.ApplyRegistration(operatorutils.
		NewRegistrationBuilder().
		WithName(anyAppName))
	commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment(envName1, envName2).
		WithEnvironment(anyOrphanedSecret, "feature"))
	commonTestUtils.ApplyDeployment(operatorutils.
		NewDeploymentBuilder().
		WithAppName(anyAppName).
		WithEnvironment(envName1).
		WithImageTag("someimageindev"))
	commonTestUtils.ApplyDeployment(operatorutils.
		NewDeploymentBuilder().
		WithAppName(anyAppName).
		WithEnvironment(anyOrphanedSecret).
		WithImageTag("someimageinfeature"))

	// Remove feature environment from application config
	commonTestUtils.ApplyApplicationUpdate(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment(envName1, envName2))

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments", anyAppName))
	response := <-responseChannel
	environments := make([]*environmentModels.EnvironmentSummary, 0)
	controllertest.GetResponseBody(response, &environments)

	for _, environment := range environments {
		if strings.EqualFold(environment.Name, anyOrphanedSecret) {
			assert.Equal(t, environmentModels.Orphan.String(), environment.Status)
			assert.NotNil(t, environment.ActiveDeployment)
		}
	}
}

func TestGetEnvironmentSummary_OrphanedSecretWithDash_OrphanedSecretIsListedOk(t *testing.T) {
	anyOrphanedSecret := "feature-1"

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _ := setupTest()
	rr, _ := commonTestUtils.ApplyRegistration(operatorutils.
		NewRegistrationBuilder().
		WithName(anyAppName))
	commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment, "master"))
	commonTestUtils.ApplyEnvironment(operatorutils.
		NewEnvironmentBuilder().
		WithAppLabel().
		WithAppName(anyAppName).
		WithEnvironmentName(anyOrphanedSecret).
		WithRegistrationOwner(rr).
		WithOrphaned(true))

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments", anyAppName))
	response := <-responseChannel
	environments := make([]*environmentModels.EnvironmentSummary, 0)
	controllertest.GetResponseBody(response, &environments)

	environmentListed := false
	for _, environment := range environments {
		if strings.EqualFold(environment.Name, anyOrphanedSecret) {
			assert.Equal(t, environmentModels.Orphan.String(), environment.Status)
			environmentListed = true
		}
	}

	assert.True(t, environmentListed)
}

func TestGetSecret_ExistingSecretInConfig_ReturnsAPendingSecret(t *testing.T) {
	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _ := setupTest()
	commonTestUtils.ApplyApplication(operatorutils.
		ARadixApplication().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment, "master"))

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyEnvironment))
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)

	environment := environmentModels.Environment{}
	err := controllertest.GetResponseBody(response, &environment)
	assert.Nil(t, err)
	assert.Equal(t, anyEnvironment, environment.Name)
	assert.Equal(t, environmentModels.Pending.String(), environment.Status)
}

func TestCreateSecret(t *testing.T) {
	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _ := setupTest()
	commonTestUtils.ApplyApplication(operatorutils.
		ARadixApplication().
		WithAppName(anyAppName))

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyEnvironment))
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
}

func Test_GetEnvironmentEvents_Handler(t *testing.T) {
	commonTestUtils, _, _, kubeclient, radixclient, _, secretproviderclient := setupTest()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	eventHandler := eventMock.NewMockEventHandler(ctrl)
	handler := initHandler(kubeclient, radixclient, secretproviderclient, WithEventHandler(eventHandler))
	raBuilder := operatorutils.ARadixApplication().WithAppName(anyAppName).WithEnvironment(anyEnvironment, "master")
	commonTestUtils.ApplyApplication(raBuilder)
	nsFunc := event.RadixEnvironmentNamespace(raBuilder.BuildRA(), anyEnvironment)
	eventHandler.EXPECT().
		GetEvents(controllertest.EqualsNamespaceFunc(nsFunc)).
		Return(make([]*eventModels.Event, 0), fmt.Errorf("err")).
		Return([]*eventModels.Event{{}, {}}, nil).
		Times(1)

	events, err := handler.GetEnvironmentEvents(anyAppName, anyEnvironment)
	assert.Nil(t, err)
	assert.Len(t, events, 2)
}

func TestRestartAuxiliaryResource(t *testing.T) {
	auxType := "oauth"

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, kubeClient, _, _, _ := setupTest()
	commonTestUtils.ApplyRegistration(operatorutils.
		NewRegistrationBuilder().
		WithName(anyAppName))
	commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment, "master"))
	commonTestUtils.ApplyDeployment(operatorutils.
		NewDeploymentBuilder().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithComponent(operatorutils.
			NewDeployComponentBuilder().
			WithName(anyComponentName).
			WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{}})).
		WithActiveFrom(time.Now()))

	envNs := operatorutils.GetEnvironmentNamespace(anyAppName, anyEnvironment)
	kubeClient.AppsV1().Deployments(envNs).Create(context.Background(), &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "comp1-aux-resource",
			Labels: map[string]string{kube.RadixAppLabel: anyAppName, kube.RadixAuxiliaryComponentLabel: anyComponentName, kube.RadixAuxiliaryComponentTypeLabel: auxType},
		},
		Spec:   appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{}}},
		Status: appsv1.DeploymentStatus{Replicas: 1},
	}, metav1.CreateOptions{})

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/aux/%s/restart", anyAppName, anyEnvironment, anyComponentName, auxType))
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)

	kubeDeploy, _ := kubeClient.AppsV1().Deployments(envNs).Get(context.Background(), "comp1-aux-resource", metav1.GetOptions{})
	assert.NotEmpty(t, kubeDeploy.Spec.Template.Annotations[restartedAtAnnotation])
}

func initHandler(client kubernetes.Interface,
	radixclient radixclient.Interface,
	secretproviderclient secretsstorevclient.Interface,
	handlerConfig ...EnvironmentHandlerOptions) EnvironmentHandler {
	accounts := models.NewAccounts(client, radixclient, secretproviderclient, nil, client, radixclient, secretproviderclient, nil, "", radixmodels.Impersonation{})
	options := []EnvironmentHandlerOptions{WithAccounts(accounts)}
	options = append(options, handlerConfig...)
	return Init(options...)
}

type ComponentCreatorStruct struct {
	name   string
	number int
}

func createRadixDeploymentWithReplicas(tu *commontest.Utils, appName, envName string, components []ComponentCreatorStruct) (*v1.RadixDeployment, error) {
	var comps []operatorutils.DeployComponentBuilder
	for _, component := range components {
		comps = append(
			comps,
			operatorutils.
				NewDeployComponentBuilder().
				WithName(component.name).
				WithReplicas(numbers.IntPtr(component.number)),
		)
	}

	rd, err := tu.ApplyDeployment(
		operatorutils.
			ARadixDeployment().
			WithComponents(comps...).
			WithAppName(appName).
			WithAnnotations(make(map[string]string)).
			WithEnvironment(envName),
	)

	return rd, err
}

func createComponentPod(kubeclient kubernetes.Interface, namespace, componentName string) {
	podSpec := getPodSpec(componentName)
	kubeclient.CoreV1().Pods(namespace).Create(context.TODO(), podSpec, metav1.CreateOptions{})
}

func deleteComponentPod(kubeclient kubernetes.Interface, namespace, componentName string) {
	kubeclient.CoreV1().Pods(namespace).Delete(context.TODO(), getComponentPodName(componentName), metav1.DeleteOptions{})
}

func findComponentInDeployment(rd *v1.RadixDeployment, componentName string) *v1.RadixDeployComponent {
	for _, comp := range rd.Spec.Components {
		if comp.Name == componentName {
			return &comp
		}
	}

	return nil
}

func getPodSpec(componentName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: getComponentPodName(componentName),
			Labels: map[string]string{
				kube.RadixComponentLabel: componentName,
			},
		},
	}
}

func getComponentPodName(componentName string) string {
	return fmt.Sprintf("%s-pod", componentName)
}

func contains(secrets []secretModels.Secret, name string) bool {
	for _, secret := range secrets {
		if secret.Name == name {
			return true
		}
	}
	return false
}
