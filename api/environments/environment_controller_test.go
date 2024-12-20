package environments

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	certclientfake "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned/fake"
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	environmentModels "github.com/equinor/radix-api/api/environments/models"
	eventModels "github.com/equinor/radix-api/api/events/models"
	"github.com/equinor/radix-api/api/secrets"
	secretModels "github.com/equinor/radix-api/api/secrets/models"
	"github.com/equinor/radix-api/api/secrets/suffix"
	controllertest "github.com/equinor/radix-api/api/test"
	"github.com/equinor/radix-api/api/utils"
	authnmock "github.com/equinor/radix-api/api/utils/token/mock"
	radixhttp "github.com/equinor/radix-common/net/http"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/numbers"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	operatordefaults "github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	kedav2 "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	prometheusclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	authorizationapiv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	testing2 "k8s.io/client-go/testing"
	secretsstorevclient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

const (
	clusterName      = "AnyClusterName"
	anyAppName       = "any-app"
	anyComponentName = "app"
	anyJobName       = "job"
	anyBatchName     = "batch"
	anyDeployment    = "deployment"
	anyEnvironment   = "dev"
	anySecretName    = "TEST_SECRET"
	egressIps        = "0.0.0.0"
	subscriptionId   = "12347718-c8f8-4995-bfbb-02655ff1f89c"
)

func setupTest(t *testing.T, envHandlerOpts []EnvironmentHandlerOptions) (*commontest.Utils, *controllertest.Utils, *controllertest.Utils, *kubefake.Clientset, radixclient.Interface, kedav2.Interface, prometheusclient.Interface, secretsstorevclient.Interface, *certclientfake.Clientset) {
	// Setup
	kubeclient := kubefake.NewClientset()
	radixClient := radixfake.NewSimpleClientset()
	kedaClient := kedafake.NewSimpleClientset()
	prometheusclient := prometheusfake.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	certClient := certclientfake.NewSimpleClientset()

	// commonTestUtils is used for creating CRDs
	commonTestUtils := commontest.NewTestUtils(kubeclient, radixClient, kedaClient, secretproviderclient)
	err := commonTestUtils.CreateClusterPrerequisites(clusterName, egressIps, subscriptionId)
	require.NoError(t, err)

	mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
	mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).AnyTimes().Return(controllertest.NewTestPrincipal(true), nil)
	// secretControllerTestUtils is used for issuing HTTP request and processing responses
	secretControllerTestUtils := controllertest.NewTestUtils(kubeclient, radixClient, kedaClient, secretproviderclient, certClient, mockValidator, secrets.NewSecretController(nil))
	// controllerTestUtils is used for issuing HTTP request and processing responses
	environmentControllerTestUtils := controllertest.NewTestUtils(kubeclient, radixClient, kedaClient, secretproviderclient, certClient, mockValidator, NewEnvironmentController(NewEnvironmentHandlerFactory(envHandlerOpts...)))

	return &commonTestUtils, &environmentControllerTestUtils, &secretControllerTestUtils, kubeclient, radixClient, kedaClient, prometheusclient, secretproviderclient, certClient
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
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, nil)
	setupGetDeploymentsTest(t, commonTestUtils, anyAppName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated, anyEnvironment)

	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/deployments", anyAppName, anyEnvironment))
	response := <-responseChannel

	deployments := make([]*deploymentModels.DeploymentSummary, 0)
	err := controllertest.GetResponseBody(response, &deployments)
	require.NoError(t, err)
	assert.Equal(t, 3, len(deployments))

	assert.Equal(t, deploymentThreeImage, deployments[0].Name)
	assert.Equal(t, deploymentThreeCreated, deployments[0].ActiveFrom)
	assert.Nil(t, deployments[0].ActiveTo)

	assert.Equal(t, deploymentTwoImage, deployments[1].Name)
	assert.Equal(t, deploymentTwoCreated, deployments[1].ActiveFrom)
	assert.Equal(t, &deploymentThreeCreated, deployments[1].ActiveTo)

	assert.Equal(t, deploymentOneImage, deployments[2].Name)
	assert.Equal(t, deploymentOneCreated, deployments[2].ActiveFrom)
	assert.Equal(t, &deploymentTwoCreated, deployments[2].ActiveTo)
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
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, nil)
	setupGetDeploymentsTest(t, commonTestUtils, anyAppName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated, anyEnvironment)

	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/deployments?latest=true", anyAppName, anyEnvironment))
	response := <-responseChannel

	deployments := make([]*deploymentModels.DeploymentSummary, 0)
	err := controllertest.GetResponseBody(response, &deployments)
	require.NoError(t, err)
	assert.Equal(t, 1, len(deployments))

	assert.Equal(t, deploymentThreeImage, deployments[0].Name)
	assert.Equal(t, deploymentThreeCreated, deployments[0].ActiveFrom)
	assert.Nil(t, deployments[0].ActiveTo)
}

func TestGetEnvironmentSummary_ApplicationWithNoDeployments_EnvironmentPending(t *testing.T) {
	envName1, envName2 := "dev", "master"

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithRadixRegistration(operatorutils.ARadixRegistration()).
		WithAppName(anyAppName).
		WithEnvironment(envName1, envName2))
	require.NoError(t, err)

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments", anyAppName))
	response := <-responseChannel
	environments := make([]*environmentModels.EnvironmentSummary, 0)
	err = controllertest.GetResponseBody(response, &environments)
	require.NoError(t, err)
	assert.Equal(t, 1, len(environments))

	assert.Equal(t, envName1, environments[0].Name)
	assert.Equal(t, environmentModels.Pending.String(), environments[0].Status)
	assert.Equal(t, envName2, environments[0].BranchMapping)
	assert.Nil(t, environments[0].ActiveDeployment)
}

func TestGetEnvironmentSummary_ApplicationWithDeployment_EnvironmentConsistent(t *testing.T) {
	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.
			ARadixDeployment().
			WithRadixApplication(operatorutils.
				NewRadixApplicationBuilder().
				WithRadixRegistration(operatorutils.ARadixRegistration()).
				WithAppName(anyAppName).
				WithEnvironment(anyEnvironment, "master")).
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment))
	require.NoError(t, err)

	re, err := radixClient.RadixV1().RadixEnvironments().Get(context.Background(), operatorutils.GetEnvironmentNamespace(anyAppName, anyEnvironment), metav1.GetOptions{})
	require.NoError(t, err)
	re.Status.Reconciled = metav1.Now()
	_, err = radixClient.RadixV1().RadixEnvironments().UpdateStatus(context.Background(), re, metav1.UpdateOptions{})
	require.NoError(t, err)

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments", anyAppName))
	response := <-responseChannel
	environments := make([]*environmentModels.EnvironmentSummary, 0)
	err = controllertest.GetResponseBody(response, &environments)
	require.NoError(t, err)

	assert.Equal(t, environmentModels.Consistent.String(), environments[0].Status)
	assert.NotNil(t, environments[0].ActiveDeployment)
}

func TestGetEnvironmentSummary_RemoveEnvironmentFromConfig_OrphanedEnvironment(t *testing.T) {
	envName1, envName2 := "dev", "master"
	anyOrphanedEnvironment := "feature-1"

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(operatorutils.
		NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment(envName1, envName2).
		WithEnvironment(anyOrphanedEnvironment, "feature"))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.
			NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(envName1).
			WithImageTag("someimageindev"))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.
			NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(anyOrphanedEnvironment).
			WithImageTag("someimageinfeature"))
	require.NoError(t, err)

	// Remove feature environment from application config
	_, err = commonTestUtils.ApplyApplicationUpdate(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment(envName1, envName2))
	require.NoError(t, err)

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments", anyAppName))
	response := <-responseChannel
	environments := make([]*environmentModels.EnvironmentSummary, 0)
	err = controllertest.GetResponseBody(response, &environments)
	require.NoError(t, err)

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
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, nil)
	rr, err := commonTestUtils.ApplyRegistration(operatorutils.
		NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment, "master"))
	require.NoError(t, err)
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyEnvironment(operatorutils.
		NewEnvironmentBuilder().
		WithAppLabel().
		WithAppName(anyAppName).
		WithEnvironmentName(anyOrphanedEnvironment).
		WithRegistrationOwner(rr).
		WithOrphaned(true))
	require.NoError(t, err)

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments", anyAppName))
	response := <-responseChannel
	environments := make([]*environmentModels.EnvironmentSummary, 0)
	err = controllertest.GetResponseBody(response, &environments)
	require.NoError(t, err)
	require.NoError(t, err)

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
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment(anyNonOrphanedEnvironment, "master").
		WithRadixRegistration(operatorutils.
			NewRegistrationBuilder().
			WithName(anyAppName)))
	require.NoError(t, err)
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyEnvironment(operatorutils.
		NewEnvironmentBuilder().
		WithAppLabel().
		WithAppName(anyAppName).
		WithEnvironmentName(anyOrphanedEnvironment).
		WithOrphaned(true))
	require.NoError(t, err)

	// Test
	// Start with two environments
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments", anyAppName))
	response := <-responseChannel
	environments := make([]*environmentModels.EnvironmentSummary, 0)
	err = controllertest.GetResponseBody(response, &environments)
	require.NoError(t, err)
	require.NoError(t, err)
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
	err = controllertest.GetResponseBody(response, &environments)
	require.NoError(t, err)
	require.NoError(t, err)
	assert.Equal(t, 1, len(environments))
}

func TestGetEnvironment_NoExistingEnvironment_ReturnsAnError(t *testing.T) {
	anyNonExistingEnvironment := "non-existing-environment"

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyApplication(operatorutils.
		ARadixApplication().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment, "master"))
	require.NoError(t, err)
	require.NoError(t, err)

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
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyApplication(operatorutils.
		ARadixApplication().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment, "master"))
	require.NoError(t, err)
	require.NoError(t, err)

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyEnvironment))
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)

	environment := environmentModels.Environment{}
	err = controllertest.GetResponseBody(response, &environment)
	require.NoError(t, err)
	require.NoError(t, err)
	assert.Equal(t, anyEnvironment, environment.Name)
	assert.Equal(t, environmentModels.Pending.String(), environment.Status)
}

func setupGetDeploymentsTest(t *testing.T, commonTestUtils *commontest.Utils, appName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage string, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated time.Time, environment string) {
	_, err := commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.
			ARadixDeployment().
			WithDeploymentName(deploymentOneImage).
			WithAppName(appName).
			WithEnvironment(environment).
			WithImageTag(deploymentOneImage).
			WithCreated(deploymentOneCreated).
			WithCondition(v1.DeploymentInactive).
			WithActiveFrom(deploymentOneCreated).
			WithActiveTo(deploymentTwoCreated))
	require.NoError(t, err)
	require.NoError(t, err)

	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.
			ARadixDeployment().
			WithDeploymentName(deploymentTwoImage).
			WithAppName(appName).
			WithEnvironment(environment).
			WithImageTag(deploymentTwoImage).
			WithCreated(deploymentTwoCreated).
			WithCondition(v1.DeploymentInactive).
			WithActiveFrom(deploymentTwoCreated).
			WithActiveTo(deploymentThreeCreated))
	require.NoError(t, err)
	require.NoError(t, err)

	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.
			ARadixDeployment().
			WithDeploymentName(deploymentThreeImage).
			WithAppName(appName).
			WithEnvironment(environment).
			WithImageTag(deploymentThreeImage).
			WithCreated(deploymentThreeCreated).
			WithCondition(v1.DeploymentActive).
			WithActiveFrom(deploymentThreeCreated))
	require.NoError(t, err)
	require.NoError(t, err)
}

func TestComponentStatusActions(t *testing.T) {

	scenarios := []ComponentCreatorStruct{
		{scenarioName: "Stop unstopped component", number: 1, name: "comp1", action: "stop", status: deploymentModels.ConsistentComponent, expectedStatus: http.StatusOK},
		{scenarioName: "Stop stopped component", number: 1, name: "comp2", action: "stop", status: deploymentModels.StoppedComponent, expectedStatus: http.StatusBadRequest},
		{scenarioName: "Start stopped component", number: 1, name: "comp3", action: "start", status: deploymentModels.StoppedComponent, expectedStatus: http.StatusOK},
		{scenarioName: "Start started component", number: 1, name: "comp4", action: "start", status: deploymentModels.ConsistentComponent, expectedStatus: http.StatusOK},
		{scenarioName: "Restart started component", number: 1, name: "comp5", action: "restart", status: deploymentModels.ConsistentComponent, expectedStatus: http.StatusOK},
		{scenarioName: "Restart stopped component", number: 1, name: "comp6", action: "restart", status: deploymentModels.StoppedComponent, expectedStatus: http.StatusBadRequest},
		{scenarioName: "Reset manually scaled component", number: 1, name: "comp7", action: "reset-scale", status: deploymentModels.StoppedComponent, expectedStatus: http.StatusOK},
	}

	// Mock Status
	statuser := func(component v1.RadixCommonDeployComponent, kd *appsv1.Deployment, rd *v1.RadixDeployment) deploymentModels.ComponentStatus {
		for _, scenario := range scenarios {
			if scenario.name == component.GetName() {
				return scenario.status
			}
		}

		panic("unknown component! ")
	}

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, []EnvironmentHandlerOptions{WithComponentStatuserFunc(statuser)})
	_, _ = createRadixDeploymentWithReplicas(commonTestUtils, anyAppName, anyEnvironment, scenarios)

	for _, scenario := range scenarios {
		t.Run(scenario.scenarioName, func(t *testing.T) {
			responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/%s", anyAppName, anyEnvironment, scenario.name, scenario.action))
			response := <-responseChannel
			assert.Equal(t, scenario.expectedStatus, response.Code)
		})
	}
}

func TestRestartEnvrionment_ApplicationWithDeployment_EnvironmentConsistent(t *testing.T) {
	zeroReplicas := 0

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, radixclient, _, _, _, _ := setupTest(t, nil)

	// Test
	t.Run("Restart Environment", func(t *testing.T) {
		envName := "fullyRunningEnv"
		rd, _ := createRadixDeploymentWithReplicas(commonTestUtils, anyAppName, envName, []ComponentCreatorStruct{
			{name: "runningComponent1", number: 1},
			{name: "runningComponent2", number: 2},
		})
		for _, comp := range rd.Spec.Components {
			assert.True(t, *comp.Replicas != zeroReplicas)
		}

		responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/restart", anyAppName, envName))
		response := <-responseChannel
		assert.Equal(t, http.StatusOK, response.Code)

		updatedRd, _ := radixclient.RadixV1().RadixDeployments(rd.GetNamespace()).Get(context.Background(), rd.GetName(), metav1.GetOptions{})
		for _, comp := range updatedRd.Spec.Components {
			assert.True(t, *comp.Replicas > zeroReplicas)
		}
	})

	t.Run("Restart Environment with stopped component", func(t *testing.T) {
		envName := "partiallyRunningEnv"
		rd, _ := createRadixDeploymentWithReplicas(commonTestUtils, anyAppName, envName, []ComponentCreatorStruct{
			{name: "stoppedComponent", number: 0},
			{name: "runningComponent", number: 7},
		})
		replicaCount := 0
		for _, comp := range rd.Spec.Components {
			replicaCount += *comp.Replicas
		}
		assert.True(t, replicaCount > zeroReplicas)

		responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/restart", anyAppName, envName))
		response := <-responseChannel
		assert.Equal(t, http.StatusOK, response.Code)

		errorResponse, _ := controllertest.GetErrorResponse(response)
		assert.Nil(t, errorResponse)

		updatedRd, _ := radixclient.RadixV1().RadixDeployments(rd.GetNamespace()).Get(context.Background(), rd.GetName(), metav1.GetOptions{})
		updatedReplicaCount := 0
		for _, comp := range updatedRd.Spec.Components {
			updatedReplicaCount += *comp.Replicas
		}
		assert.True(t, updatedReplicaCount == replicaCount)
	})
}

func TestCreateEnvironment(t *testing.T) {
	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyApplication(operatorutils.
		ARadixApplication().
		WithAppName(anyAppName))
	require.NoError(t, err)

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyEnvironment))
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
}

func Test_GetEnvironmentEvents_Controller(t *testing.T) {
	envName := "dev"

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, kubeClient, _, _, _, _, _ := setupTest(t, nil)
	createEvent := func(namespace, eventName string) {
		_, err := kubeClient.CoreV1().Events(namespace).CreateWithEventNamespace(&corev1.Event{
			ObjectMeta: metav1.ObjectMeta{
				Name: eventName,
			},
		})
		require.NoError(t, err)
	}
	createEvent(operatorutils.GetEnvironmentNamespace(anyAppName, envName), "ev1")
	createEvent(operatorutils.GetEnvironmentNamespace(anyAppName, envName), "ev2")
	_, err := commonTestUtils.ApplyApplication(operatorutils.
		ARadixApplication().
		WithAppName(anyAppName).
		WithEnvironment(envName, "master"))
	require.NoError(t, err)

	t.Run("Get events for dev environment", func(t *testing.T) {
		responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/events", anyAppName, envName))
		response := <-responseChannel
		assert.Equal(t, http.StatusOK, response.Code)
		events := make([]eventModels.Event, 0)
		err = controllertest.GetResponseBody(response, &events)
		require.NoError(t, err)
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

func TestUpdateSecret_AccountSecretForComponentVolumeMount_UpdatedOk(t *testing.T) {
	// Setup
	commonTestUtils, environmentControllerTestUtils, controllerTestUtils, client, radixclient, kedaClient, promclient, secretProviderClient, certClient := setupTest(t, nil)
	err := utils.ApplyDeploymentWithSync(client, radixclient, kedaClient, promclient, commonTestUtils, secretProviderClient, certClient, operatorutils.ARadixDeployment().
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
	require.NoError(t, err)

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyEnvironment))
	response := <-responseChannel

	environment := environmentModels.Environment{}
	err = controllertest.GetResponseBody(response, &environment)
	require.NoError(t, err)
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
	commonTestUtils, environmentControllerTestUtils, controllerTestUtils, client, radixclient, kedaClient, promclient, secretProviderClient, certClient := setupTest(t, nil)
	err := utils.ApplyDeploymentWithSync(client, radixclient, kedaClient, promclient, commonTestUtils, secretProviderClient, certClient, operatorutils.ARadixDeployment().
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
	require.NoError(t, err)

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyEnvironment))
	response := <-responseChannel

	environment := environmentModels.Environment{}
	err = controllertest.GetResponseBody(response, &environment)
	require.NoError(t, err)
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
	commonTestUtils, environmentControllerTestUtils, controllerTestUtils, client, radixclient, kedaClient, promclient, secretProviderClient, certClient := setupTest(t, nil)
	err := utils.ApplyDeploymentWithSync(client, radixclient, kedaClient, promclient, commonTestUtils, secretProviderClient, certClient, operatorutils.NewDeploymentBuilder().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithRadixApplication(operatorutils.ARadixApplication().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment, "master")).
		WithComponents(
			operatorutils.NewDeployComponentBuilder().WithName(anyComponentName).WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{SessionStoreType: v1.SessionStoreRedis}}),
		))
	require.NoError(t, err)

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyEnvironment))
	response := <-responseChannel

	environment := environmentModels.Environment{}
	err = controllertest.GetResponseBody(response, &environment)
	require.NoError(t, err)
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
	secretName := operatorutils.GetAuxiliaryComponentSecretName(anyComponentName, operatordefaults.OAuthProxyAuxiliaryComponentSuffix)
	_, err = client.CoreV1().Secrets(envNs).Create(context.Background(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName}}, metav1.CreateOptions{})
	require.NoError(t, err)

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters(
		"PUT",
		fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/secrets/%s", anyAppName, anyEnvironment, anyComponentName, anyComponentName+suffix.OAuth2ClientSecret),
		secretModels.SecretParameters{SecretValue: "clientsecret"},
	)
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
	actualSecret, _ := client.CoreV1().Secrets(envNs).Get(context.Background(), secretName, metav1.GetOptions{})
	assert.Equal(t, actualSecret.Data, map[string][]byte{operatordefaults.OAuthClientSecretKeyName: []byte("clientsecret")})

	// Update client secret when k8s secret exists should set Data
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters(
		"PUT",
		fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/secrets/%s", anyAppName, anyEnvironment, anyComponentName, anyComponentName+suffix.OAuth2CookieSecret),
		secretModels.SecretParameters{SecretValue: "cookiesecret"},
	)
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
	actualSecret, _ = client.CoreV1().Secrets(envNs).Get(context.Background(), secretName, metav1.GetOptions{})
	assert.Equal(t, actualSecret.Data, map[string][]byte{operatordefaults.OAuthClientSecretKeyName: []byte("clientsecret"), operatordefaults.OAuthCookieSecretKeyName: []byte("cookiesecret")})

	// Update client secret when k8s secret exists should set Data
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters(
		"PUT",
		fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/secrets/%s", anyAppName, anyEnvironment, anyComponentName, anyComponentName+suffix.OAuth2RedisPassword),
		secretModels.SecretParameters{SecretValue: "redispassword"},
	)
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
	actualSecret, _ = client.CoreV1().Secrets(envNs).Get(context.Background(), secretName, metav1.GetOptions{})
	assert.Equal(t, actualSecret.Data, map[string][]byte{operatordefaults.OAuthClientSecretKeyName: []byte("clientsecret"), operatordefaults.OAuthCookieSecretKeyName: []byte("cookiesecret"), operatordefaults.OAuthRedisPasswordKeyName: []byte("redispassword")})
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
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, nil)
	setupGetDeploymentsTest(t, commonTestUtils, anyAppName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated, anyEnvironment)

	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/deployments", anyAppName, anyEnvironment))
	response := <-responseChannel

	deployments := make([]*deploymentModels.DeploymentSummary, 0)
	err := controllertest.GetResponseBody(response, &deployments)
	require.NoError(t, err)
	assert.Equal(t, 3, len(deployments))

	assert.Equal(t, deploymentThreeImage, deployments[0].Name)
	assert.Equal(t, deploymentThreeCreated, deployments[0].ActiveFrom)
	assert.Nil(t, deployments[0].ActiveTo)

	assert.Equal(t, deploymentTwoImage, deployments[1].Name)
	assert.Equal(t, deploymentTwoCreated, deployments[1].ActiveFrom)
	assert.Equal(t, &deploymentThreeCreated, deployments[1].ActiveTo)

	assert.Equal(t, deploymentOneImage, deployments[2].Name)
	assert.Equal(t, deploymentOneCreated, deployments[2].ActiveFrom)
	assert.Equal(t, &deploymentTwoCreated, deployments[2].ActiveTo)
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
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, nil)
	setupGetDeploymentsTest(t, commonTestUtils, anyAppName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated, anyEnvironment)

	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/deployments?latest=true", anyAppName, anyEnvironment))
	response := <-responseChannel

	deployments := make([]*deploymentModels.DeploymentSummary, 0)
	err := controllertest.GetResponseBody(response, &deployments)
	require.NoError(t, err)
	assert.Equal(t, 1, len(deployments))

	assert.Equal(t, deploymentThreeImage, deployments[0].Name)
	assert.Equal(t, deploymentThreeCreated, deployments[0].ActiveFrom)
	assert.Nil(t, deployments[0].ActiveTo)
}

func TestGetEnvironmentSummary_ApplicationWithNoDeployments_SecretPending(t *testing.T) {
	envName1, envName2 := "dev", "master"

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithRadixRegistration(operatorutils.ARadixRegistration()).
		WithAppName(anyAppName).
		WithEnvironment(envName1, envName2))
	require.NoError(t, err)

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments", anyAppName))
	response := <-responseChannel
	environments := make([]*environmentModels.EnvironmentSummary, 0)
	err = controllertest.GetResponseBody(response, &environments)
	require.NoError(t, err)

	assert.Equal(t, 1, len(environments))
	assert.Equal(t, envName1, environments[0].Name)
	assert.Equal(t, environmentModels.Pending.String(), environments[0].Status)
	assert.Equal(t, envName2, environments[0].BranchMapping)
	assert.Nil(t, environments[0].ActiveDeployment)
}

func TestGetEnvironmentSummary_RemoveSecretFromConfig_OrphanedSecret(t *testing.T) {
	envName1, envName2 := "dev", "master"
	orphanedEnvironment := "feature-1"

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(operatorutils.
		NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment(envName1, envName2).
		WithEnvironment(orphanedEnvironment, "feature"))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.
			NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(envName1).
			WithImageTag("someimageindev"))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.
			NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(orphanedEnvironment).
			WithImageTag("someimageinfeature"))
	require.NoError(t, err)

	// Remove feature environment from application config
	_, err = commonTestUtils.ApplyApplicationUpdate(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment(envName1, envName2))
	require.NoError(t, err)

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments", anyAppName))
	response := <-responseChannel
	environments := make([]*environmentModels.EnvironmentSummary, 0)
	err = controllertest.GetResponseBody(response, &environments)
	require.NoError(t, err)

	for _, environment := range environments {
		if strings.EqualFold(environment.Name, orphanedEnvironment) {
			assert.Equal(t, environmentModels.Orphan.String(), environment.Status)
			assert.NotNil(t, environment.ActiveDeployment)
		}
	}
}

func TestGetEnvironmentSummary_OrphanedSecretWithDash_OrphanedSecretIsListedOk(t *testing.T) {
	orphanedEnvironment := "feature-1"

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, nil)
	rr, err := commonTestUtils.ApplyRegistration(operatorutils.
		NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment, "master"))
	require.NoError(t, err)
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyEnvironment(operatorutils.
		NewEnvironmentBuilder().
		WithAppLabel().
		WithAppName(anyAppName).
		WithEnvironmentName(orphanedEnvironment).
		WithRegistrationOwner(rr).
		WithOrphaned(true))
	require.NoError(t, err)

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments", anyAppName))
	response := <-responseChannel
	environments := make([]*environmentModels.EnvironmentSummary, 0)
	err = controllertest.GetResponseBody(response, &environments)
	require.NoError(t, err)

	environmentListed := false
	for _, environment := range environments {
		if strings.EqualFold(environment.Name, orphanedEnvironment) {
			assert.Equal(t, environmentModels.Orphan.String(), environment.Status)
			environmentListed = true
		}
	}

	assert.True(t, environmentListed)
}

func TestGetSecret_ExistingSecretInConfig_ReturnsAPendingSecret(t *testing.T) {
	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyApplication(operatorutils.
		ARadixApplication().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment, "master"))
	require.NoError(t, err)

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyEnvironment))
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)

	environment := environmentModels.Environment{}
	err = controllertest.GetResponseBody(response, &environment)
	require.NoError(t, err)
	assert.Nil(t, err)
	assert.Equal(t, anyEnvironment, environment.Name)
	assert.Equal(t, environmentModels.Pending.String(), environment.Status)
}

func TestCreateSecret(t *testing.T) {
	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyApplication(operatorutils.
		ARadixApplication().
		WithAppName(anyAppName))
	require.NoError(t, err)

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyEnvironment))
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
}

func TestRestartAuxiliaryResource(t *testing.T) {
	auxType := "oauth"
	called := 0

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, kubeClient, _, _, _, _, _ := setupTest(t, nil)
	kubeClient.Fake.PrependReactor("create", "*", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
		createAction, ok := action.DeepCopy().(testing2.CreateAction)
		if !ok {
			return false, nil, nil
		}

		review, ok := createAction.GetObject().(*authorizationapiv1.SelfSubjectAccessReview)
		if !ok {
			return false, nil, nil
		}

		called++

		if review.Spec.ResourceAttributes.Name != anyAppName {
			return true, review, nil
		}

		assert.Equal(t, review.Spec.ResourceAttributes.Name, anyAppName)
		assert.Equal(t, review.Spec.ResourceAttributes.Resource, v1.ResourceRadixRegistrations)
		assert.Equal(t, review.Spec.ResourceAttributes.Verb, "patch")

		review.Status.Allowed = true
		return true, review, nil
	})
	_, err := commonTestUtils.ApplyRegistration(operatorutils.
		NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyRegistration(operatorutils.
		NewRegistrationBuilder().
		WithName("forbidden"))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment, "master"))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.
			NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithComponent(operatorutils.
				NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{}})).
			WithActiveFrom(time.Now()))
	require.NoError(t, err)

	envNs := operatorutils.GetEnvironmentNamespace(anyAppName, anyEnvironment)
	_, err = kubeClient.AppsV1().Deployments(envNs).Create(context.Background(), &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "comp1-aux-resource",
			Labels: map[string]string{kube.RadixAppLabel: anyAppName, kube.RadixAuxiliaryComponentLabel: anyComponentName, kube.RadixAuxiliaryComponentTypeLabel: auxType},
		},
		Spec:   appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{}}},
		Status: appsv1.DeploymentStatus{Replicas: 1},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	// Test
	responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/aux/%s/restart", anyAppName, anyEnvironment, anyComponentName, auxType))
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, 1, called)

	kubeDeploy, _ := kubeClient.AppsV1().Deployments(envNs).Get(context.Background(), "comp1-aux-resource", metav1.GetOptions{})
	assert.NotEmpty(t, kubeDeploy.Spec.Template.Annotations[restartedAtAnnotation])

	// Test Forbidden for other app names

	responseChannel = environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/aux/%s/restart", "forbidden", anyEnvironment, anyComponentName, auxType))
	response = <-responseChannel
	assert.Equal(t, http.StatusForbidden, response.Code)
	assert.Equal(t, 2, called)
}

func Test_GetJobs(t *testing.T) {
	batch1Name, batch2Name := "batch1", "batch2"
	namespace := operatorutils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(operatorutils.
		NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.
			NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithJobComponents(operatorutils.NewDeployJobComponentBuilder().WithName(anyJobName)).
			WithActiveFrom(time.Now()))
	require.NoError(t, err)

	// Insert test data
	testData := []v1.RadixBatch{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   batch1Name,
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeJob)),
			},
			Spec: v1.RadixBatchSpec{Jobs: []v1.RadixBatchJob{{Name: "job1"}}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   batch2Name,
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeJob)),
			},
			Spec: v1.RadixBatchSpec{Jobs: []v1.RadixBatchJob{{Name: "job2"}}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "otherjobbatch",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName("othercomponent"), labels.ForBatchType(kube.RadixBatchTypeJob)),
			},
			Spec: v1.RadixBatchSpec{Jobs: []v1.RadixBatchJob{{Name: "anyjob"}}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "anybatch",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Spec: v1.RadixBatchSpec{Jobs: []v1.RadixBatchJob{{Name: "job3"}}},
		},
	}
	for _, rb := range testData {
		_, err := radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), &rb, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	// Test get jobs for jobComponent1Name
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs", anyAppName, anyEnvironment, anyJobName))
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
	var actual []deploymentModels.ScheduledJobSummary
	err = controllertest.GetResponseBody(response, &actual)
	require.NoError(t, err)
	require.NoError(t, err)
	require.Len(t, actual, 2)
	actualMapped := slice.Map(actual, func(job deploymentModels.ScheduledJobSummary) string {
		return job.Name
	})
	expected := []string{batch1Name + "-job1", batch2Name + "-job2"}
	assert.ElementsMatch(t, expected, actualMapped)
}

func Test_GetJobs_Status(t *testing.T) {
	namespace := operatorutils.GetEnvironmentNamespace(anyAppName, anyEnvironment)
	type scenario struct {
		name           string
		jobStatus      *v1.RadixBatchJobStatus
		expectedStatus deploymentModels.ScheduledBatchJobStatus
	}
	scenarios := []scenario{
		{
			name:           "no job status",
			expectedStatus: v1.RadixBatchJobApiStatusWaiting,
		},
		{
			name: "pod is pending, no phase",
			jobStatus: &v1.RadixBatchJobStatus{
				Name:                     "no1",
				RadixBatchJobPodStatuses: []v1.RadixBatchJobPodStatus{{CreationTime: &metav1.Time{Time: time.Now()}, Phase: v1.PodPending}},
			},
			expectedStatus: v1.RadixBatchJobApiStatusWaiting,
		},
		{
			name: "pod is pending, phase is waiting",
			jobStatus: &v1.RadixBatchJobStatus{
				Name:                     "no1",
				Phase:                    v1.BatchJobPhaseWaiting,
				RadixBatchJobPodStatuses: []v1.RadixBatchJobPodStatus{{CreationTime: &metav1.Time{Time: time.Now()}, Phase: v1.PodPending}},
			},
			expectedStatus: v1.RadixBatchJobApiStatusWaiting,
		},
		{
			name: "pod is running, phase is active",
			jobStatus: &v1.RadixBatchJobStatus{
				Name:                     "no1",
				Phase:                    v1.BatchJobPhaseActive,
				RadixBatchJobPodStatuses: []v1.RadixBatchJobPodStatus{{CreationTime: &metav1.Time{Time: time.Now()}, Phase: v1.PodRunning}},
			},
			expectedStatus: v1.RadixBatchJobApiStatusActive,
		},
		{
			name: "pod is suceeded, phase is succeeded",
			jobStatus: &v1.RadixBatchJobStatus{
				Name:                     "no1",
				Phase:                    v1.BatchJobPhaseSucceeded,
				RadixBatchJobPodStatuses: []v1.RadixBatchJobPodStatus{{CreationTime: &metav1.Time{Time: time.Now()}, Phase: v1.PodSucceeded}},
			},
			expectedStatus: v1.RadixBatchJobApiStatusSucceeded,
		},
		{
			name: "pod is failed, phase is failed",
			jobStatus: &v1.RadixBatchJobStatus{
				Name:                     "no1",
				Phase:                    v1.BatchJobPhaseFailed,
				RadixBatchJobPodStatuses: []v1.RadixBatchJobPodStatus{{CreationTime: &metav1.Time{Time: time.Now()}, Phase: v1.PodFailed}},
			},
			expectedStatus: v1.RadixBatchJobApiStatusFailed,
		},
		{
			name: "pod is succeeded, pphase is stopped",
			jobStatus: &v1.RadixBatchJobStatus{
				Name:                     "no1",
				Phase:                    v1.BatchJobPhaseStopped,
				RadixBatchJobPodStatuses: []v1.RadixBatchJobPodStatus{{CreationTime: &metav1.Time{Time: time.Now()}, Phase: v1.PodSucceeded}},
			},
			expectedStatus: v1.RadixBatchJobApiStatusStopped,
		},
		{
			name:           "no pod status, phase is not defined",
			jobStatus:      &v1.RadixBatchJobStatus{Name: "not-defined"},
			expectedStatus: v1.RadixBatchJobApiStatusWaiting,
		},
	}
	for _, ts := range scenarios {
		t.Run(ts.name, func(t *testing.T) {
			// Setup
			commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, []EnvironmentHandlerOptions{})
			_, err := commonTestUtils.ApplyRegistration(operatorutils.
				NewRegistrationBuilder().
				WithName(anyAppName))
			require.NoError(t, err)
			_, err = commonTestUtils.ApplyApplication(operatorutils.
				NewRadixApplicationBuilder().
				WithAppName(anyAppName))
			require.NoError(t, err)
			_, err = commonTestUtils.ApplyDeployment(
				context.Background(),
				operatorutils.
					NewDeploymentBuilder().
					WithAppName(anyAppName).
					WithEnvironment(anyEnvironment).
					WithJobComponents(operatorutils.NewDeployJobComponentBuilder().WithName(anyJobName)).
					WithActiveFrom(time.Now()))
			require.NoError(t, err)

			// Insert test data
			batch := v1.RadixBatch{
				ObjectMeta: metav1.ObjectMeta{
					Name:   anyBatchName,
					Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeJob)),
				},
				Spec: v1.RadixBatchSpec{
					Jobs: []v1.RadixBatchJob{{Name: "no1"}}},
				Status: v1.RadixBatchStatus{},
			}
			if ts.jobStatus != nil {
				batch.Status.JobStatuses = append(batch.Status.JobStatuses, *ts.jobStatus)
			}
			_, err = radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), &batch, metav1.CreateOptions{})
			require.NoError(t, err)

			// Test get jobs for jobComponent1Name
			responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs", anyAppName, anyEnvironment, anyJobName))
			response := <-responseChannel
			assert.Equal(t, http.StatusOK, response.Code)
			var actual []deploymentModels.ScheduledJobSummary
			err = controllertest.GetResponseBody(response, &actual)
			require.NoError(t, err)
			assert.Len(t, actual, 1)
			assert.Equal(t, ts.expectedStatus, actual[0].Status)
		})
	}

}

func Test_GetBatch_JobsListStatus_StopIsTrue(t *testing.T) {
	namespace := operatorutils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(operatorutils.
		NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.
			NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithJobComponents(operatorutils.NewDeployJobComponentBuilder().WithName(anyJobName)).
			WithActiveFrom(time.Now()))
	require.NoError(t, err)

	// Insert test data
	testData := []v1.RadixBatch{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   anyBatchName,
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Spec: v1.RadixBatchSpec{
				Jobs: []v1.RadixBatchJob{
					{Name: "no1", Stop: radixutils.BoolPtr(true)},
					{Name: "no2", Stop: radixutils.BoolPtr(true)},
					{Name: "no3", Stop: radixutils.BoolPtr(true)},
					{Name: "no4", Stop: radixutils.BoolPtr(true)},
					{Name: "no5", Stop: radixutils.BoolPtr(true)},
					{Name: "no6", Stop: radixutils.BoolPtr(true)},
					{Name: "no7", Stop: radixutils.BoolPtr(true)},
				},
			},
			Status: v1.RadixBatchStatus{
				JobStatuses: []v1.RadixBatchJobStatus{
					{Name: "no2"},
					{Name: "no3", Phase: v1.BatchJobPhaseWaiting},
					{Name: "no4", Phase: v1.BatchJobPhaseActive},
					{Name: "no5", Phase: v1.BatchJobPhaseSucceeded},
					{Name: "no6", Phase: v1.BatchJobPhaseFailed},
					{Name: "no7", Phase: v1.BatchJobPhaseStopped},
					{Name: "not-defined"},
				},
			},
		},
	}
	for _, rb := range testData {
		_, err := radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), &rb, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	// Test get jobs for jobComponent1Name
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/batches", anyAppName, anyEnvironment, anyJobName))
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
	var actual []deploymentModels.ScheduledBatchSummary
	err = controllertest.GetResponseBody(response, &actual)
	require.NoError(t, err)
	require.Len(t, actual, 1)
	type assertMapped struct {
		Name   string
		Status deploymentModels.ScheduledBatchJobStatus
	}
	actualMapped := slice.Map(actual[0].JobList, func(job deploymentModels.ScheduledJobSummary) assertMapped {
		return assertMapped{Name: job.Name, Status: job.Status}
	})
	expected := []assertMapped{
		{Name: anyBatchName + "-no1", Status: utils.GetBatchJobStatusByJobApiStatus(v1.RadixBatchJobApiStatusStopping)},
		{Name: anyBatchName + "-no2", Status: utils.GetBatchJobStatusByJobApiStatus(v1.RadixBatchJobApiStatusStopping)},
		{Name: anyBatchName + "-no3", Status: utils.GetBatchJobStatusByJobApiStatus(v1.RadixBatchJobApiStatusStopping)},
		{Name: anyBatchName + "-no4", Status: utils.GetBatchJobStatusByJobApiStatus(v1.RadixBatchJobApiStatusStopping)},
		{Name: anyBatchName + "-no5", Status: utils.GetBatchJobStatusByJobApiStatus(v1.RadixBatchJobApiStatusSucceeded)},
		{Name: anyBatchName + "-no6", Status: utils.GetBatchJobStatusByJobApiStatus(v1.RadixBatchJobApiStatusFailed)},
		{Name: anyBatchName + "-no7", Status: utils.GetBatchJobStatusByJobApiStatus(v1.RadixBatchJobApiStatusStopped)},
	}
	assert.ElementsMatch(t, expected, actualMapped)
}

func Test_GetSingleJobs_Status_StopIsTrue(t *testing.T) {
	namespace := operatorutils.GetEnvironmentNamespace(anyAppName, anyEnvironment)
	type scenario struct {
		name            string
		batchStatusType v1.RadixBatchConditionType
		jobStatus       *v1.RadixBatchJobStatus
		expectedStatus  deploymentModels.ScheduledBatchJobStatus
	}
	scenarios := []scenario{
		{name: "No status",
			jobStatus:      nil,
			expectedStatus: v1.RadixBatchJobApiStatusStopping,
		},
		{
			name:            "No phase",
			batchStatusType: v1.BatchConditionTypeWaiting,
			jobStatus:       &v1.RadixBatchJobStatus{Name: "no1"},
			expectedStatus:  deploymentModels.ScheduledBatchJobStatusStopping,
		},
		{
			name:            "In waiting",
			batchStatusType: v1.BatchConditionTypeWaiting,
			jobStatus:       &v1.RadixBatchJobStatus{Name: "no1", Phase: v1.BatchJobPhaseWaiting},
			expectedStatus:  deploymentModels.ScheduledBatchJobStatusStopping,
		},
		{
			name:            "Active",
			batchStatusType: v1.BatchConditionTypeActive,
			jobStatus:       &v1.RadixBatchJobStatus{Name: "no1", Phase: v1.BatchJobPhaseActive},
			expectedStatus:  deploymentModels.ScheduledBatchJobStatusStopping,
		},
		{
			name:            "Succeeded",
			batchStatusType: v1.BatchConditionTypeCompleted,
			jobStatus:       &v1.RadixBatchJobStatus{Name: "no1", Phase: v1.BatchJobPhaseSucceeded},
			expectedStatus:  deploymentModels.ScheduledBatchJobStatusSucceeded,
		},
		{
			name:            "Failed",
			batchStatusType: v1.BatchConditionTypeCompleted,
			jobStatus:       &v1.RadixBatchJobStatus{Name: "no1", Phase: v1.BatchJobPhaseFailed},
			expectedStatus:  deploymentModels.ScheduledBatchJobStatusFailed,
		},
		{
			name:            "Stopped",
			batchStatusType: v1.BatchConditionTypeCompleted,
			jobStatus:       &v1.RadixBatchJobStatus{Name: "no1", Phase: v1.BatchJobPhaseStopped},
			expectedStatus:  deploymentModels.ScheduledBatchJobStatusStopped,
		},
	}
	for _, ts := range scenarios {
		t.Run(ts.name, func(t *testing.T) {
			// Setup
			commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, nil)
			_, err := commonTestUtils.ApplyRegistration(operatorutils.
				NewRegistrationBuilder().
				WithName(anyAppName))
			require.NoError(t, err)
			_, err = commonTestUtils.ApplyApplication(operatorutils.
				NewRadixApplicationBuilder().
				WithAppName(anyAppName))
			require.NoError(t, err)
			_, err = commonTestUtils.ApplyDeployment(
				context.Background(),
				operatorutils.
					NewDeploymentBuilder().
					WithAppName(anyAppName).
					WithEnvironment(anyEnvironment).
					WithJobComponents(operatorutils.NewDeployJobComponentBuilder().WithName(anyJobName)).
					WithActiveFrom(time.Now()))
			require.NoError(t, err)

			// Insert test data
			batch := v1.RadixBatch{
				ObjectMeta: metav1.ObjectMeta{
					Name:   anyBatchName,
					Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeJob)),
				},
				Spec: v1.RadixBatchSpec{
					Jobs: []v1.RadixBatchJob{
						{Name: "no1", Stop: radixutils.BoolPtr(true)},
					},
				},
				Status: v1.RadixBatchStatus{
					Condition: v1.RadixBatchCondition{
						Type: v1.BatchConditionTypeActive,
					},
				},
			}
			if ts.jobStatus != nil {
				batch.Status.JobStatuses = append(batch.Status.JobStatuses, *ts.jobStatus)
			}
			_, err = radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), &batch, metav1.CreateOptions{})
			require.NoError(t, err)

			// Test get jobs for jobComponent1Name
			responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs", anyAppName, anyEnvironment, anyJobName))
			response := <-responseChannel
			assert.Equal(t, http.StatusOK, response.Code)
			var actual []deploymentModels.ScheduledJobSummary
			err = controllertest.GetResponseBody(response, &actual)
			require.NoError(t, err)
			assert.Len(t, actual, 1)
			assert.Equal(t, ts.expectedStatus, actual[0].Status)
		})

	}
}

func Test_GetJob(t *testing.T) {
	namespace := operatorutils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(operatorutils.
		NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.
			NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithJobComponents(operatorutils.NewDeployJobComponentBuilder().WithName(anyJobName)).
			WithActiveFrom(time.Now()))
	require.NoError(t, err)

	// Insert test data
	testData := []v1.RadixBatch{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "job-batch1",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeJob)),
			},
			Spec: v1.RadixBatchSpec{
				Jobs: []v1.RadixBatchJob{{Name: "job1"}, {Name: "job2"}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "job-batch2",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Spec: v1.RadixBatchSpec{
				Jobs: []v1.RadixBatchJob{{Name: "job1"}, {Name: "job2"}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "other-batch1",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName("other-component"), labels.ForBatchType(kube.RadixBatchTypeJob)),
			},
			Spec: v1.RadixBatchSpec{
				Jobs: []v1.RadixBatchJob{{Name: "job1"}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "other-batch2",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName("other-component"), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Spec: v1.RadixBatchSpec{
				Jobs: []v1.RadixBatchJob{{Name: "job1"}},
			},
		},
	}
	for _, rb := range testData {
		_, err := radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), &rb, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	type scenarioSpec struct {
		Name    string
		JobName string
		Success bool
	}

	scenarions := []scenarioSpec{
		{Name: "get existing job1 from existing batch of type job", JobName: "job-batch1-job1", Success: true},
		{Name: "get existing job2 from existing batch of type job", JobName: "job-batch1-job2", Success: true},
		{Name: "get non-existing job3 from existing batch of type job", JobName: "job-batch1-job3", Success: false},
		{Name: "get existing job from existing batch of type job for other jobcomponent", JobName: "other-batch1-job1", Success: false},
		{Name: "get existing job1 from existing batch of type batch", JobName: "job-batch2-job1", Success: true},
		{Name: "get existing job2 from existing batch of type batch", JobName: "job-batch2-job2", Success: true},
		{Name: "get non-existing job3 from existing batch of type batch", JobName: "job-batch2-job3", Success: false},
		{Name: "get existing job from existing batch of type batch for other jobcomponent", JobName: "other-batch2-job1", Success: false},
		{Name: "get job from non-existing batch", JobName: "non-existing-batch-anyjob", Success: false},
	}

	for _, scenario := range scenarions {
		scenario := scenario
		t.Run(scenario.Name, func(t *testing.T) {
			responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs/%s", anyAppName, anyEnvironment, anyJobName, scenario.JobName))
			response := <-responseChannel
			assert.Equal(t, scenario.Success, response.Code == http.StatusOK)
		})
	}
}

func Test_GetJob_AllProps(t *testing.T) {
	namespace := operatorutils.GetEnvironmentNamespace(anyAppName, anyEnvironment)
	creationTime := metav1.NewTime(time.Date(2022, 1, 2, 3, 4, 5, 0, time.UTC))
	startTime := metav1.NewTime(time.Date(2022, 1, 2, 3, 4, 10, 0, time.UTC))
	podCreationTime := metav1.NewTime(time.Date(2022, 1, 2, 3, 4, 15, 0, time.UTC))
	endTime := metav1.NewTime(time.Date(2022, 1, 2, 3, 4, 15, 0, time.UTC))
	defaultBackoffLimit := numbers.Int32Ptr(3)

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(operatorutils.
		NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.
			NewDeploymentBuilder().
			WithDeploymentName(anyDeployment).
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithJobComponents(operatorutils.
				NewDeployJobComponentBuilder().
				WithName(anyJobName).
				WithTimeLimitSeconds(numbers.Int64Ptr(123)).
				WithNodeGpu("gpu1").
				WithNodeGpuCount("2").
				WithRuntime(&v1.Runtime{Architecture: v1.RuntimeArchitectureArm64}).
				WithResource(map[string]string{"cpu": "50Mi", "memory": "250M"}, map[string]string{"cpu": "100Mi", "memory": "500M"})).
			WithActiveFrom(time.Now()))
	require.NoError(t, err)

	// HACK: Missing WithBackoffLimit in DeploymentBuild, so we''' have to update the RD manually
	rd, _ := radixClient.RadixV1().RadixDeployments(namespace).Get(context.Background(), anyDeployment, metav1.GetOptions{})
	rd.Spec.Jobs[0].BackoffLimit = defaultBackoffLimit
	_, err = radixClient.RadixV1().RadixDeployments(namespace).Update(context.Background(), rd, metav1.UpdateOptions{})
	require.NoError(t, err)

	// Insert test data
	testData := []v1.RadixBatch{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "job-batch1",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeJob)),
			},
			Spec: v1.RadixBatchSpec{
				Jobs: []v1.RadixBatchJob{
					{
						Name: "job1",
					},
					{
						Name:             "job2",
						JobId:            "anyjobid",
						BackoffLimit:     numbers.Int32Ptr(5),
						TimeLimitSeconds: numbers.Int64Ptr(999),
						Resources: &v1.ResourceRequirements{
							Limits:   v1.ResourceList{"cpu": "101Mi", "memory": "501M"},
							Requests: v1.ResourceList{"cpu": "51Mi", "memory": "251M"},
						},
						Node: &v1.RadixNode{
							Gpu:      "gpu2",
							GpuCount: "3",
						},
					},
				},
				RadixDeploymentJobRef: v1.RadixDeploymentJobComponentSelector{
					Job:                  anyJobName,
					LocalObjectReference: v1.LocalObjectReference{Name: anyDeployment},
				},
			},
			Status: v1.RadixBatchStatus{
				Condition: v1.RadixBatchCondition{
					Type: v1.BatchConditionTypeCompleted,
				},
				JobStatuses: []v1.RadixBatchJobStatus{
					{
						Name:         "job1",
						Phase:        v1.BatchJobPhaseSucceeded,
						Message:      "anymessage",
						CreationTime: &creationTime,
						StartTime:    &startTime,
						EndTime:      &endTime,
						RadixBatchJobPodStatuses: []v1.RadixBatchJobPodStatus{{
							CreationTime: &podCreationTime,
							Phase:        v1.PodSucceeded,
						}},
					},
				},
			},
		},
	}
	for _, rb := range testData {
		_, err := radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), &rb, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	// Test job1 props - props from RD jobComponent
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs/%s", anyAppName, anyEnvironment, anyJobName, "job-batch1-job1"))
	response := <-responseChannel
	var actual deploymentModels.ScheduledJobSummary
	err = controllertest.GetResponseBody(response, &actual)
	require.NoError(t, err)
	assert.Equal(t, deploymentModels.ScheduledJobSummary{
		Name:             "job-batch1-job1",
		Created:          &creationTime.Time,
		Started:          &startTime.Time,
		Ended:            &endTime.Time,
		Status:           deploymentModels.ScheduledBatchJobStatusSucceeded,
		Message:          "anymessage",
		BackoffLimit:     *defaultBackoffLimit,
		TimeLimitSeconds: numbers.Int64Ptr(123),
		Resources: deploymentModels.ResourceRequirements{
			Limits:   deploymentModels.Resources{CPU: "100Mi", Memory: "500M"},
			Requests: deploymentModels.Resources{CPU: "50Mi", Memory: "250M"},
		},
		Node:           &deploymentModels.Node{Gpu: "gpu1", GpuCount: "2"},
		DeploymentName: anyDeployment,
		ReplicaList: []deploymentModels.ReplicaSummary{{
			Created: podCreationTime.Time,
			Status:  deploymentModels.ReplicaStatus{Status: deploymentModels.Succeeded},
		}},
		Runtime: &deploymentModels.Runtime{
			Architecture: string(v1.RuntimeArchitectureArm64),
		},
	}, actual)

	// Test job2 props - override props from RD jobComponent
	responseChannel = environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs/%s", anyAppName, anyEnvironment, anyJobName, "job-batch1-job2"))
	response = <-responseChannel
	actual = deploymentModels.ScheduledJobSummary{}
	err = controllertest.GetResponseBody(response, &actual)
	require.NoError(t, err)
	assert.Equal(t, deploymentModels.ScheduledJobSummary{
		Created:          nil,
		Name:             "job-batch1-job2",
		JobId:            "anyjobid",
		Status:           deploymentModels.ScheduledBatchJobStatusWaiting,
		BackoffLimit:     5,
		TimeLimitSeconds: numbers.Int64Ptr(999),
		Resources: deploymentModels.ResourceRequirements{
			Limits:   deploymentModels.Resources{CPU: "101Mi", Memory: "501M"},
			Requests: deploymentModels.Resources{CPU: "51Mi", Memory: "251M"},
		},
		Node:           &deploymentModels.Node{Gpu: "gpu2", GpuCount: "3"},
		DeploymentName: anyDeployment,
		Runtime: &deploymentModels.Runtime{
			Architecture: string(v1.RuntimeArchitectureArm64),
		},
	}, actual)
}

func Test_GetJobPayload(t *testing.T) {
	namespace := operatorutils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, kubeClient, radixClient, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(operatorutils.
		NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.
			NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithJobComponents(operatorutils.
				NewDeployJobComponentBuilder().
				WithName(anyJobName)).
			WithActiveFrom(time.Now()))
	require.NoError(t, err)

	// Insert test data
	rb := v1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "job-batch1",
			Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeJob)),
		},
		Spec: v1.RadixBatchSpec{
			Jobs: []v1.RadixBatchJob{
				{Name: "job1"},
				{Name: "job2", PayloadSecretRef: &v1.PayloadSecretKeySelector{
					Key:                  "payload1",
					LocalObjectReference: v1.LocalObjectReference{Name: anySecretName},
				}},
				{Name: "job3", PayloadSecretRef: &v1.PayloadSecretKeySelector{
					Key:                  "missingpayloadkey",
					LocalObjectReference: v1.LocalObjectReference{Name: anySecretName},
				}},
				{Name: "job4", PayloadSecretRef: &v1.PayloadSecretKeySelector{
					Key:                  "payload1",
					LocalObjectReference: v1.LocalObjectReference{Name: "otherSecret"},
				}},
			},
		}}
	_, err = radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), &rb, metav1.CreateOptions{})
	require.NoError(t, err)

	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: anySecretName},
		Data: map[string][]byte{
			"payload1": []byte("job1payload"),
		},
	}
	_, err = kubeClient.CoreV1().Secrets(namespace).Create(context.Background(), &secret, metav1.CreateOptions{})
	require.NoError(t, err)

	// Test job1 payload
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs/%s/payload", anyAppName, anyEnvironment, anyJobName, "job-batch1-job1"))
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Empty(t, response.Body.Bytes())

	// Test job2 payload
	responseChannel = environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs/%s/payload", anyAppName, anyEnvironment, anyJobName, "job-batch1-job2"))
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "job1payload", response.Body.String())

	// Test job3 payload
	responseChannel = environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs/%s/payload", anyAppName, anyEnvironment, anyJobName, "job-batch1-job3"))
	response = <-responseChannel
	assert.Equal(t, http.StatusNotFound, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	assert.Equal(t, environmentModels.ScheduledJobPayloadNotFoundError(anyAppName, "job-batch1-job3"), errorResponse)

	// Test job4 payload
	responseChannel = environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs/%s/payload", anyAppName, anyEnvironment, anyJobName, "job-batch1-job4"))
	response = <-responseChannel
	assert.Equal(t, http.StatusNotFound, response.Code)
	errorResponse, _ = controllertest.GetErrorResponse(response)
	assert.Equal(t, environmentModels.ScheduledJobPayloadNotFoundError(anyAppName, "job-batch1-job4"), errorResponse)

}

func Test_GetBatch_JobList(t *testing.T) {
	namespace := operatorutils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(operatorutils.
		NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.
			NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithJobComponents(operatorutils.NewDeployJobComponentBuilder().WithName(anyJobName)).
			WithActiveFrom(time.Now()))
	require.NoError(t, err)

	// Insert test data
	testData := []v1.RadixBatch{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   anyBatchName,
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Spec: v1.RadixBatchSpec{
				Jobs: []v1.RadixBatchJob{{Name: "no1"}, {Name: "no2"}, {Name: "no3"}, {Name: "no4"}, {Name: "no5"}, {Name: "no6"}, {Name: "no7"}, {Name: "no8"}}},
			Status: v1.RadixBatchStatus{
				JobStatuses: []v1.RadixBatchJobStatus{
					{Name: "no2"},
					{Name: "no3", Phase: v1.BatchJobPhaseWaiting},
					{Name: "no4", Phase: v1.BatchJobPhaseActive},
					{Name: "no5", Phase: v1.BatchJobPhaseRunning},
					{Name: "no6", Phase: v1.BatchJobPhaseSucceeded},
					{Name: "no7", Phase: v1.BatchJobPhaseFailed},
					{Name: "no8", Phase: v1.BatchJobPhaseStopped},
					{Name: "not-defined"},
				},
			},
		},
	}
	for _, rb := range testData {
		_, err := radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), &rb, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	// Test get jobs for jobComponent1Name
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/batches/%s", anyAppName, anyEnvironment, anyJobName, anyBatchName))
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
	var actual deploymentModels.ScheduledBatchSummary
	err = controllertest.GetResponseBody(response, &actual)
	require.NoError(t, err)
	require.Len(t, actual.JobList, 8)
	type assertMapped struct {
		Name   string
		Status deploymentModels.ScheduledBatchJobStatus
	}
	actualMapped := slice.Map(actual.JobList, func(job deploymentModels.ScheduledJobSummary) assertMapped {
		return assertMapped{Name: job.Name, Status: job.Status}
	})
	expected := []assertMapped{
		{Name: anyBatchName + "-no1", Status: deploymentModels.ScheduledBatchJobStatusWaiting},
		{Name: anyBatchName + "-no2", Status: deploymentModels.ScheduledBatchJobStatusWaiting},
		{Name: anyBatchName + "-no3", Status: deploymentModels.ScheduledBatchJobStatusWaiting},
		{Name: anyBatchName + "-no4", Status: deploymentModels.ScheduledBatchJobStatusActive},
		{Name: anyBatchName + "-no5", Status: deploymentModels.ScheduledBatchJobStatusRunning},
		{Name: anyBatchName + "-no6", Status: deploymentModels.ScheduledBatchJobStatusSucceeded},
		{Name: anyBatchName + "-no7", Status: deploymentModels.ScheduledBatchJobStatusFailed},
		{Name: anyBatchName + "-no8", Status: deploymentModels.ScheduledBatchJobStatusStopped},
	}
	assert.ElementsMatch(t, expected, actualMapped)
}

func Test_GetBatch_JobList_StopFlag(t *testing.T) {
	namespace := operatorutils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(operatorutils.
		NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.
			NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithJobComponents(operatorutils.NewDeployJobComponentBuilder().WithName(anyJobName)).
			WithActiveFrom(time.Now()))
	require.NoError(t, err)

	// Insert test data
	testData := []v1.RadixBatch{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   anyBatchName,
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Spec: v1.RadixBatchSpec{
				Jobs: []v1.RadixBatchJob{{Name: "no1", Stop: radixutils.BoolPtr(true)}, {Name: "no2", Stop: radixutils.BoolPtr(true)}, {Name: "no3", Stop: radixutils.BoolPtr(true)}, {Name: "no4", Stop: radixutils.BoolPtr(true)}, {Name: "no5", Stop: radixutils.BoolPtr(true)}, {Name: "no6", Stop: radixutils.BoolPtr(true)}, {Name: "no7", Stop: radixutils.BoolPtr(true)}}},
			Status: v1.RadixBatchStatus{
				JobStatuses: []v1.RadixBatchJobStatus{
					{Name: "no2"},
					{Name: "no3", Phase: v1.BatchJobPhaseWaiting},
					{Name: "no4", Phase: v1.BatchJobPhaseActive},
					{Name: "no5", Phase: v1.BatchJobPhaseSucceeded},
					{Name: "no6", Phase: v1.BatchJobPhaseFailed},
					{Name: "no7", Phase: v1.BatchJobPhaseStopped},
					{Name: "not-defined"},
				},
			},
		},
	}
	for _, rb := range testData {
		_, err := radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), &rb, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	// Test get jobs for jobComponent1Name
	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/batches/%s", anyAppName, anyEnvironment, anyJobName, anyBatchName))
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
	var actual deploymentModels.ScheduledBatchSummary
	err = controllertest.GetResponseBody(response, &actual)
	require.NoError(t, err)
	require.Len(t, actual.JobList, 7)
	type assertMapped struct {
		Name   string
		Status deploymentModels.ScheduledBatchJobStatus
	}
	actualMapped := slice.Map(actual.JobList, func(job deploymentModels.ScheduledJobSummary) assertMapped {
		return assertMapped{Name: job.Name, Status: job.Status}
	})
	expected := []assertMapped{
		{Name: anyBatchName + "-no1", Status: deploymentModels.ScheduledBatchJobStatusStopping},
		{Name: anyBatchName + "-no2", Status: deploymentModels.ScheduledBatchJobStatusStopping},
		{Name: anyBatchName + "-no3", Status: deploymentModels.ScheduledBatchJobStatusStopping},
		{Name: anyBatchName + "-no4", Status: deploymentModels.ScheduledBatchJobStatusStopping},
		{Name: anyBatchName + "-no5", Status: deploymentModels.ScheduledBatchJobStatusSucceeded},
		{Name: anyBatchName + "-no6", Status: deploymentModels.ScheduledBatchJobStatusFailed},
		{Name: anyBatchName + "-no7", Status: deploymentModels.ScheduledBatchJobStatusStopped},
	}
	assert.ElementsMatch(t, expected, actualMapped)
}

func Test_GetBatches_Status(t *testing.T) {
	namespace := operatorutils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(operatorutils.
		NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.
			NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithJobComponents(operatorutils.NewDeployJobComponentBuilder().WithName(anyJobName)).
			WithActiveFrom(time.Now()))
	require.NoError(t, err)

	// Insert test data
	testData := []v1.RadixBatch{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "batch-job1",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "batch-job2",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Status: v1.RadixBatchStatus{
				Condition: v1.RadixBatchCondition{Type: v1.BatchConditionTypeWaiting},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "batch-job3",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Status: v1.RadixBatchStatus{
				JobStatuses: []v1.RadixBatchJobStatus{
					{Name: "j1"},
					{
						Name:  "j2",
						Phase: v1.BatchJobPhaseActive,
						RadixBatchJobPodStatuses: []v1.RadixBatchJobPodStatus{{
							Phase:        v1.PodRunning,
							CreationTime: &metav1.Time{Time: time.Now()},
							StartTime:    &metav1.Time{Time: time.Now()},
						}},
					},
					{
						Name:  "j3",
						Phase: v1.BatchJobPhaseWaiting,
						RadixBatchJobPodStatuses: []v1.RadixBatchJobPodStatus{{
							Phase:        v1.PodPending,
							CreationTime: &metav1.Time{Time: time.Now()},
						}},
					},
				},
				Condition: v1.RadixBatchCondition{Type: v1.BatchConditionTypeActive},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "batch-job4",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Status: v1.RadixBatchStatus{
				JobStatuses: []v1.RadixBatchJobStatus{
					{Name: "j1"},
					{
						Name:  "j2",
						Phase: v1.BatchJobPhaseRunning,
						RadixBatchJobPodStatuses: []v1.RadixBatchJobPodStatus{{
							Phase:        v1.PodRunning,
							CreationTime: &metav1.Time{Time: time.Now()},
							StartTime:    &metav1.Time{Time: time.Now()},
						}},
					},
					{
						Name:  "j3",
						Phase: v1.BatchJobPhaseSucceeded,
						RadixBatchJobPodStatuses: []v1.RadixBatchJobPodStatus{{
							Phase:        v1.PodSucceeded,
							CreationTime: &metav1.Time{Time: time.Now()},
							StartTime:    &metav1.Time{Time: time.Now()},
							EndTime:      &metav1.Time{Time: time.Now()},
						}},
					},
				},
				Condition: v1.RadixBatchCondition{Type: v1.BatchConditionTypeActive},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "batch-job5",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Status: v1.RadixBatchStatus{
				Condition: v1.RadixBatchCondition{Type: v1.BatchConditionTypeCompleted},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "batch-job6",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Status: v1.RadixBatchStatus{
				Condition: v1.RadixBatchCondition{Type: v1.BatchConditionTypeCompleted},
				JobStatuses: []v1.RadixBatchJobStatus{
					{
						Name:    "j1",
						Phase:   v1.BatchJobPhaseFailed,
						EndTime: &metav1.Time{Time: time.Now()},
						Failed:  1,
						RadixBatchJobPodStatuses: []v1.RadixBatchJobPodStatus{{
							Phase:        v1.PodFailed,
							CreationTime: &metav1.Time{Time: time.Now()},
							StartTime:    &metav1.Time{Time: time.Now()},
							EndTime:      &metav1.Time{Time: time.Now()},
						}},
					},
					{
						Name:    "j2",
						Phase:   v1.BatchJobPhaseFailed,
						EndTime: &metav1.Time{Time: time.Now()},
						Failed:  1,
						RadixBatchJobPodStatuses: []v1.RadixBatchJobPodStatus{{
							Phase:        v1.PodFailed,
							CreationTime: &metav1.Time{Time: time.Now()},
							StartTime:    &metav1.Time{Time: time.Now()},
							EndTime:      &metav1.Time{Time: time.Now()},
						}},
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "batch-job7",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Status: v1.RadixBatchStatus{
				Condition: v1.RadixBatchCondition{Type: v1.BatchConditionTypeCompleted},
				JobStatuses: []v1.RadixBatchJobStatus{
					{
						Name:    "j1",
						Phase:   v1.BatchJobPhaseFailed,
						EndTime: &metav1.Time{Time: time.Now()},
						Failed:  1,
						RadixBatchJobPodStatuses: []v1.RadixBatchJobPodStatus{{
							Phase:        v1.PodFailed,
							CreationTime: &metav1.Time{Time: time.Now()},
							StartTime:    &metav1.Time{Time: time.Now()},
							EndTime:      &metav1.Time{Time: time.Now()},
						}},
					},
					{
						Name:    "j2",
						Phase:   v1.BatchJobPhaseSucceeded,
						EndTime: &metav1.Time{Time: time.Now()},
						RadixBatchJobPodStatuses: []v1.RadixBatchJobPodStatus{{
							Phase:        v1.PodSucceeded,
							CreationTime: &metav1.Time{Time: time.Now()},
							StartTime:    &metav1.Time{Time: time.Now()},
							EndTime:      &metav1.Time{Time: time.Now()},
						}},
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "batch-compute1",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName("other-component"), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "jobtype-job1",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeJob)),
			},
		},
	}
	for _, rb := range testData {
		_, err := radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), &rb, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/batches", anyAppName, anyEnvironment, anyJobName))
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
	var actual []deploymentModels.ScheduledBatchSummary
	err = controllertest.GetResponseBody(response, &actual)
	require.NoError(t, err)
	type assertMapped struct {
		Name   string
		Status deploymentModels.ScheduledBatchJobStatus
	}
	actualMapped := slice.Map(actual, func(b deploymentModels.ScheduledBatchSummary) assertMapped {
		return assertMapped{Name: b.Name, Status: b.Status}
	})
	expected := []assertMapped{
		{Name: "batch-job1", Status: deploymentModels.ScheduledBatchJobStatusWaiting},
		{Name: "batch-job2", Status: deploymentModels.ScheduledBatchJobStatusWaiting},
		{Name: "batch-job3", Status: deploymentModels.ScheduledBatchJobStatusActive},
		{Name: "batch-job4", Status: deploymentModels.ScheduledBatchJobStatusActive},
		{Name: "batch-job5", Status: deploymentModels.ScheduledBatchJobStatusCompleted},
		{Name: "batch-job6", Status: deploymentModels.ScheduledBatchJobStatusCompleted},
		{Name: "batch-job7", Status: deploymentModels.ScheduledBatchJobStatusCompleted},
	}
	assert.ElementsMatch(t, expected, actualMapped)
}

func Test_GetBatches_JobListShouldHaveJobWithStatusWaiting(t *testing.T) {
	namespace := operatorutils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(operatorutils.
		NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.
			NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithJobComponents(operatorutils.NewDeployJobComponentBuilder().WithName(anyJobName)).
			WithActiveFrom(time.Now()))
	require.NoError(t, err)

	// Insert test data
	testData := []v1.RadixBatch{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "batch-job1",
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Spec: v1.RadixBatchSpec{
				Jobs: []v1.RadixBatchJob{{Name: "j1"}},
			},
		},
	}
	for _, rb := range testData {
		_, err := radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), &rb, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	responseChannel := environmentControllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/batches", anyAppName, anyEnvironment, anyJobName))
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)

	var actual []deploymentModels.ScheduledBatchSummary
	err = controllertest.GetResponseBody(response, &actual)
	require.NoError(t, err)
	require.Len(t, actual, 1)
	assert.Len(t, actual[0].JobList, 1)
	assert.Equal(t, deploymentModels.ScheduledBatchJobStatusWaiting, actual[0].JobList[0].Status)
}

func Test_StopJob(t *testing.T) {
	type JobTestData struct {
		name      string
		jobStatus v1.RadixBatchJobStatus
	}

	batchTypeBatchName, batchTypeJobName := "batchBatch", "jobBatch"
	namespace := operatorutils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	validJobs := []JobTestData{
		{name: "validJob1"},
		{name: "validJob2", jobStatus: v1.RadixBatchJobStatus{Name: "validJob2", Phase: ""}},
		{name: "validJob3", jobStatus: v1.RadixBatchJobStatus{Name: "validJob3", Phase: v1.BatchJobPhaseWaiting}},
		{name: "validJob4", jobStatus: v1.RadixBatchJobStatus{Name: "validJob4", Phase: v1.BatchJobPhaseActive}},
		{name: "validJob5", jobStatus: v1.RadixBatchJobStatus{Name: "validJob5", Phase: v1.BatchJobPhaseRunning}},
	}
	invalidJobs := []JobTestData{
		{name: "invalidJob1", jobStatus: v1.RadixBatchJobStatus{Name: "invalidJob1", Phase: v1.BatchJobPhaseSucceeded}},
		{name: "invalidJob2", jobStatus: v1.RadixBatchJobStatus{Name: "invalidJob2", Phase: v1.BatchJobPhaseFailed}},
		{name: "invalidJob3", jobStatus: v1.RadixBatchJobStatus{Name: "invalidJob3", Phase: v1.BatchJobPhaseStopped}},
	}
	nonExistentJobs := []JobTestData{
		{name: "noJob"},
	}

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(operatorutils.
		NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.
			NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithJobComponents(operatorutils.NewDeployJobComponentBuilder().WithName(anyJobName)).
			WithActiveFrom(time.Now()))
	require.NoError(t, err)

	jobSpecList := append(
		slice.Map(validJobs, func(j JobTestData) v1.RadixBatchJob { return v1.RadixBatchJob{Name: j.name} }),
		slice.Map(invalidJobs, func(j JobTestData) v1.RadixBatchJob { return v1.RadixBatchJob{Name: j.name} })...,
	)
	jobStatuses := append(
		slice.Map(validJobs, func(j JobTestData) v1.RadixBatchJobStatus { return j.jobStatus }),
		slice.Map(invalidJobs, func(j JobTestData) v1.RadixBatchJobStatus { return j.jobStatus })...,
	)
	// Insert test data
	testData := []v1.RadixBatch{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   batchTypeBatchName,
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Spec: v1.RadixBatchSpec{Jobs: jobSpecList},
			Status: v1.RadixBatchStatus{
				JobStatuses: jobStatuses,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   batchTypeJobName,
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeJob)),
			},
			Spec: v1.RadixBatchSpec{Jobs: jobSpecList},
			Status: v1.RadixBatchStatus{
				JobStatuses: jobStatuses,
			},
		},
	}
	for _, rb := range testData {
		_, err := radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), &rb, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	// jobs by name that can be stopped
	validJobNames := slice.Reduce(validJobs, []string{}, func(obj []string, job JobTestData) []string { return append(obj, job.name) })

	// Test both batches
	for _, batchName := range []string{batchTypeBatchName, batchTypeJobName} {
		// Test valid jobs
		for _, v := range validJobs {
			responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs/%s/stop", anyAppName, anyEnvironment, anyJobName, batchName+"-"+v.name))
			response := <-responseChannel
			assert.Equal(t, http.StatusNoContent, response.Code)
			assert.Empty(t, response.Body.Bytes())
		}

		// Test invalid jobs
		for _, v := range invalidJobs {
			responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs/%s/stop", anyAppName, anyEnvironment, anyJobName, batchName+"-"+v.name))
			response := <-responseChannel
			assert.Equal(t, http.StatusBadRequest, response.Code)
			assert.NotEmpty(t, response.Body.Bytes())
		}

		// Test non-existent jobs
		for _, v := range nonExistentJobs {
			responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs/%s/stop", anyAppName, anyEnvironment, anyJobName, batchName+"-"+v.name))
			response := <-responseChannel
			assert.Equal(t, http.StatusNotFound, response.Code)
			assert.NotEmpty(t, response.Body.Bytes())
		}

		// Check that stoppable jobs are stopped
		assertBatchJobStoppedStates(t, radixClient, namespace, batchName, validJobNames)
	}
}

func Test_DeleteJob(t *testing.T) {
	type JobTestData struct {
		name      string
		jobStatus v1.RadixBatchJobStatus
	}

	batchTypeBatchName, batchTypeJobNames := "batchBatch", []string{"jobBatch1", "jobBatch2", "jobBatch3"}
	namespace := operatorutils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	jobs := []JobTestData{
		{name: "validJob1"},
		{name: "validJob2", jobStatus: v1.RadixBatchJobStatus{Name: "validJob2", Phase: ""}},
		{name: "validJob3", jobStatus: v1.RadixBatchJobStatus{Name: "validJob3", Phase: v1.BatchJobPhaseWaiting}},
		{name: "validJob4", jobStatus: v1.RadixBatchJobStatus{Name: "validJob4", Phase: v1.BatchJobPhaseActive}},
	}

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(operatorutils.
		NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.
			NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithJobComponents(operatorutils.NewDeployJobComponentBuilder().WithName(anyJobName)).
			WithActiveFrom(time.Now()))
	require.NoError(t, err)

	// Insert test data
	testData := []v1.RadixBatch{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   batchTypeBatchName,
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Spec:   v1.RadixBatchSpec{Jobs: []v1.RadixBatchJob{{Name: jobs[0].name}, {Name: jobs[1].name}, {Name: jobs[2].name}, {Name: jobs[3].name}}},
			Status: v1.RadixBatchStatus{JobStatuses: []v1.RadixBatchJobStatus{jobs[0].jobStatus, jobs[1].jobStatus, jobs[2].jobStatus, jobs[3].jobStatus}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   batchTypeJobNames[0],
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeJob)),
			},
			Spec:   v1.RadixBatchSpec{Jobs: []v1.RadixBatchJob{{Name: jobs[0].name}}},
			Status: v1.RadixBatchStatus{JobStatuses: []v1.RadixBatchJobStatus{jobs[0].jobStatus}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   batchTypeJobNames[1],
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeJob)),
			},
			Spec:   v1.RadixBatchSpec{Jobs: []v1.RadixBatchJob{{Name: jobs[1].name}}},
			Status: v1.RadixBatchStatus{JobStatuses: []v1.RadixBatchJobStatus{jobs[1].jobStatus}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   batchTypeJobNames[2],
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeJob)),
			},
			Spec:   v1.RadixBatchSpec{Jobs: []v1.RadixBatchJob{{Name: jobs[2].name}}},
			Status: v1.RadixBatchStatus{JobStatuses: []v1.RadixBatchJobStatus{jobs[2].jobStatus}},
		},
	}
	for _, rb := range testData {
		_, err := radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), &rb, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	// deletable jobs
	deletableJobs := []string{batchTypeJobNames[0], batchTypeJobNames[2]} // selected jobs to delete
	for _, batchName := range deletableJobs {
		jobs := testData[slice.FindIndex(testData, func(batch v1.RadixBatch) bool { return batch.Name == batchName })].Spec.Jobs
		responseChannel := environmentControllerTestUtils.ExecuteRequest("DELETE", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs/%s", anyAppName, anyEnvironment, anyJobName, batchName+"-"+jobs[0].Name))
		response := <-responseChannel
		assert.Equal(t, http.StatusNoContent, response.Code)
		assert.Empty(t, response.Body.Bytes())
	}

	// non-deletable jobs
	nonDeletableJobs := []string{batchTypeBatchName}
	for _, batchName := range nonDeletableJobs {
		jobs := testData[slice.FindIndex(testData, func(batch v1.RadixBatch) bool { return batch.Name == batchName })].Spec.Jobs
		jobNames := slice.Reduce(jobs, []string{}, func(names []string, job v1.RadixBatchJob) []string { return append(names, job.Name) })
		for _, jobName := range jobNames {
			responseChannel := environmentControllerTestUtils.ExecuteRequest("DELETE", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs/%s", anyAppName, anyEnvironment, anyJobName, batchName+"-"+jobName))
			response := <-responseChannel
			assert.Equal(t, http.StatusNotFound, response.Code)
			assert.NotEmpty(t, response.Body.Bytes())
		}
	}

	// non-existent jobs
	nonExistentJobs := []string{"noBatch"}
	for _, batchName := range nonExistentJobs {
		jobName := "noJob"
		responseChannel := environmentControllerTestUtils.ExecuteRequest("DELETE", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/jobs/%s", anyAppName, anyEnvironment, anyJobName, batchName+"-"+jobName))
		response := <-responseChannel
		assert.Equal(t, http.StatusNotFound, response.Code)
		assert.NotEmpty(t, response.Body.Bytes())
	}

	// assert only deletable jobs are deleted/gone
	for _, batchName := range append(batchTypeJobNames, batchTypeBatchName) {
		assertBatchDeleted(t, radixClient, namespace, batchName, deletableJobs)
	}
}

func Test_StopBatch(t *testing.T) {
	type JobTestData struct {
		name      string
		jobStatus v1.RadixBatchJobStatus
	}

	batchTypeBatchName, batchTypeJobName, nonExistentBatch := "batchBatch", "jobBatch", "noBatch"
	namespace := operatorutils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	validJobs := []JobTestData{
		{name: "validJob1"},
		{name: "validJob2", jobStatus: v1.RadixBatchJobStatus{Name: "validJob2", Phase: ""}},
		{name: "validJob3", jobStatus: v1.RadixBatchJobStatus{Name: "validJob3", Phase: v1.BatchJobPhaseWaiting}},
		{name: "validJob4", jobStatus: v1.RadixBatchJobStatus{Name: "validJob4", Phase: v1.BatchJobPhaseActive}},
	}
	invalidJobs := []JobTestData{
		{name: "invalidJob1", jobStatus: v1.RadixBatchJobStatus{Name: "invalidJob1", Phase: v1.BatchJobPhaseSucceeded}},
		{name: "invalidJob2", jobStatus: v1.RadixBatchJobStatus{Name: "invalidJob2", Phase: v1.BatchJobPhaseFailed}},
		{name: "invalidJob3", jobStatus: v1.RadixBatchJobStatus{Name: "invalidJob3", Phase: v1.BatchJobPhaseStopped}},
	}

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(operatorutils.
		NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.
			NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithJobComponents(operatorutils.NewDeployJobComponentBuilder().WithName(anyJobName)).
			WithActiveFrom(time.Now()))
	require.NoError(t, err)

	// Insert test data
	testData := []v1.RadixBatch{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   batchTypeBatchName,
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Spec: v1.RadixBatchSpec{
				Jobs: []v1.RadixBatchJob{
					{Name: validJobs[0].name}, {Name: validJobs[1].name}, {Name: validJobs[2].name}, {Name: validJobs[3].name},
					{Name: invalidJobs[0].name}, {Name: invalidJobs[1].name}, {Name: invalidJobs[2].name},
				}},
			Status: v1.RadixBatchStatus{
				Condition: v1.RadixBatchCondition{Type: v1.BatchConditionTypeActive},
				JobStatuses: []v1.RadixBatchJobStatus{
					validJobs[0].jobStatus, validJobs[1].jobStatus, validJobs[2].jobStatus, validJobs[3].jobStatus,
					invalidJobs[0].jobStatus, invalidJobs[1].jobStatus, invalidJobs[2].jobStatus,
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   batchTypeJobName,
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeJob)),
			},
			Spec: v1.RadixBatchSpec{
				Jobs: []v1.RadixBatchJob{
					{Name: validJobs[0].name}, {Name: validJobs[1].name}, {Name: validJobs[2].name}, {Name: validJobs[3].name},
					{Name: invalidJobs[0].name}, {Name: invalidJobs[1].name}, {Name: invalidJobs[2].name},
				}},
			Status: v1.RadixBatchStatus{
				Condition: v1.RadixBatchCondition{Type: v1.BatchConditionTypeActive},
				JobStatuses: []v1.RadixBatchJobStatus{
					validJobs[0].jobStatus, validJobs[1].jobStatus, validJobs[2].jobStatus, validJobs[3].jobStatus,
					invalidJobs[0].jobStatus, invalidJobs[1].jobStatus, invalidJobs[2].jobStatus,
				},
			},
		},
	}
	for _, rb := range testData {
		_, err := radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), &rb, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	// Test valid batch
	responseChannel := environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/batches/%s/stop", anyAppName, anyEnvironment, anyJobName, batchTypeBatchName))
	response := <-responseChannel
	assert.Equal(t, http.StatusNoContent, response.Code)
	assert.Empty(t, response.Body.Bytes())

	// Test invalid batch type
	responseChannel = environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/batches/%s/stop", anyAppName, anyEnvironment, anyJobName, batchTypeJobName))
	response = <-responseChannel
	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.NotEmpty(t, response.Body.Bytes())

	// Test non-existent batch
	responseChannel = environmentControllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/batches/%s/stop", anyAppName, anyEnvironment, anyJobName, nonExistentBatch))
	response = <-responseChannel
	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.NotEmpty(t, response.Body.Bytes())

	// jobs by name that can be stopped
	validJobNames := slice.Reduce(validJobs, []string{}, func(obj []string, job JobTestData) []string { return append(obj, job.name) })

	// Check that stoppable jobs are stopped
	assertBatchJobStoppedStates(t, radixClient, namespace, batchTypeBatchName, validJobNames)
	assertBatchJobStoppedStates(t, radixClient, namespace, batchTypeJobName, []string{}) // invalid batch type, no jobs should be stopped
}

func Test_DeleteBatch(t *testing.T) {
	type JobTestData struct {
		name      string
		jobStatus v1.RadixBatchJobStatus
	}

	batchTypeBatchNames, batchTypeJobName := []string{"batchBatch1", "batchBatch2", "batchBatch3"}, "jobBatch"
	namespace := operatorutils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	jobs := []JobTestData{
		{name: "validJob1"},
		{name: "validJob2", jobStatus: v1.RadixBatchJobStatus{Name: "validJob2", Phase: ""}},
		{name: "validJob3", jobStatus: v1.RadixBatchJobStatus{Name: "validJob3", Phase: v1.BatchJobPhaseWaiting}},
		{name: "validJob4", jobStatus: v1.RadixBatchJobStatus{Name: "validJob4", Phase: v1.BatchJobPhaseActive}},
	}

	// Setup
	commonTestUtils, environmentControllerTestUtils, _, _, radixClient, _, _, _, _ := setupTest(t, nil)
	_, err := commonTestUtils.ApplyRegistration(operatorutils.
		NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyApplication(operatorutils.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.
			NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithJobComponents(operatorutils.NewDeployJobComponentBuilder().WithName(anyJobName)).
			WithActiveFrom(time.Now()))
	require.NoError(t, err)

	// Insert test data
	testData := []v1.RadixBatch{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   batchTypeBatchNames[0],
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Spec:   v1.RadixBatchSpec{Jobs: []v1.RadixBatchJob{{Name: jobs[0].name}, {Name: jobs[1].name}, {Name: jobs[2].name}, {Name: jobs[3].name}}},
			Status: v1.RadixBatchStatus{JobStatuses: []v1.RadixBatchJobStatus{jobs[0].jobStatus, jobs[1].jobStatus, jobs[2].jobStatus, jobs[3].jobStatus}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   batchTypeBatchNames[1],
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Spec:   v1.RadixBatchSpec{Jobs: []v1.RadixBatchJob{{Name: jobs[0].name}, {Name: jobs[1].name}}},
			Status: v1.RadixBatchStatus{JobStatuses: []v1.RadixBatchJobStatus{jobs[0].jobStatus, jobs[1].jobStatus}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   batchTypeBatchNames[2],
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeBatch)),
			},
			Spec:   v1.RadixBatchSpec{Jobs: []v1.RadixBatchJob{{Name: jobs[2].name}, {Name: jobs[3].name}}},
			Status: v1.RadixBatchStatus{JobStatuses: []v1.RadixBatchJobStatus{jobs[2].jobStatus, jobs[3].jobStatus}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   batchTypeJobName,
				Labels: labels.Merge(labels.ForApplicationName(anyAppName), labels.ForComponentName(anyJobName), labels.ForBatchType(kube.RadixBatchTypeJob)),
			},
			Spec:   v1.RadixBatchSpec{Jobs: []v1.RadixBatchJob{{Name: jobs[0].name}}},
			Status: v1.RadixBatchStatus{JobStatuses: []v1.RadixBatchJobStatus{jobs[0].jobStatus}},
		},
	}
	for _, rb := range testData {
		_, err := radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), &rb, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	// deletable batches
	deletableBatches := []string{batchTypeBatchNames[0], batchTypeBatchNames[2]} // selected jobs to delete
	for _, batchName := range deletableBatches {
		responseChannel := environmentControllerTestUtils.ExecuteRequest("DELETE", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/batches/%s", anyAppName, anyEnvironment, anyJobName, batchName))
		response := <-responseChannel
		assert.Equal(t, http.StatusNoContent, response.Code)
		assert.Empty(t, response.Body.Bytes())
	}

	// non-deletable batches
	nonDeletableJobs := []string{batchTypeJobName}
	for _, batchName := range nonDeletableJobs {
		responseChannel := environmentControllerTestUtils.ExecuteRequest("DELETE", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/batches/%s", anyAppName, anyEnvironment, anyJobName, batchName))
		response := <-responseChannel
		assert.Equal(t, http.StatusNotFound, response.Code)
		assert.NotEmpty(t, response.Body.Bytes())
	}

	// non-existent batches
	nonExistentJobs := []string{"noBatch"}
	for _, batchName := range nonExistentJobs {
		responseChannel := environmentControllerTestUtils.ExecuteRequest("DELETE", fmt.Sprintf("/api/v1/applications/%s/environments/%s/jobcomponents/%s/batches/%s", anyAppName, anyEnvironment, anyJobName, batchName))
		response := <-responseChannel
		assert.Equal(t, http.StatusNotFound, response.Code)
		assert.NotEmpty(t, response.Body.Bytes())
	}

	// assert only deletable batches are deleted/gone
	for _, batchName := range append(batchTypeBatchNames, batchTypeJobName) {
		assertBatchDeleted(t, radixClient, namespace, batchName, deletableBatches)
	}
}

type ComponentCreatorStruct struct {
	name           string
	number         int
	action         string
	status         deploymentModels.ComponentStatus
	expectedStatus int
	scenarioName   string
}

func createRadixDeploymentWithReplicas(tu *commontest.Utils, appName, envName string, components []ComponentCreatorStruct) (*v1.RadixDeployment, error) {
	var comps []operatorutils.DeployComponentBuilder
	for _, component := range components {
		comps = append(
			comps,
			operatorutils.
				NewDeployComponentBuilder().
				WithName(component.name).
				WithReplicas(pointers.Ptr(component.number)).
				WithReplicasOverride(pointers.Ptr(component.number)),
		)
	}

	rd, err := tu.ApplyDeployment(
		context.Background(),
		operatorutils.
			ARadixDeployment().
			WithComponents(comps...).
			WithAppName(appName).
			WithAnnotations(make(map[string]string)).
			WithEnvironment(envName),
	)

	return rd, err
}

func contains(secrets []secretModels.Secret, name string) bool {
	for _, secret := range secrets {
		if secret.Name == name {
			return true
		}
	}
	return false
}

func assertBatchDeleted(t *testing.T, rc radixclient.Interface, ns, batchName string, deletableBatches []string) {
	updatedBatch, err := rc.RadixV1().RadixBatches(ns).Get(context.Background(), batchName, metav1.GetOptions{})
	if slice.FindIndex(deletableBatches, func(name string) bool { return name == batchName }) == -1 {
		// deletable
		require.NotNil(t, updatedBatch)
		require.Nil(t, err)
	} else {
		// not deletable
		require.Error(t, err)
	}
}

func assertBatchJobStoppedStates(t *testing.T, rc radixclient.Interface, ns, batchName string, stoppableJobs []string) {
	updatedBatch, err := rc.RadixV1().RadixBatches(ns).Get(context.Background(), batchName, metav1.GetOptions{})
	require.Nil(t, err)

	isStopped := func(job *v1.RadixBatchJob) bool {
		if job == nil || job.Stop == nil {
			return false
		}
		return *job.Stop
	}

	for _, job := range updatedBatch.Spec.Jobs {
		if slice.FindIndex(stoppableJobs, func(name string) bool { return name == job.Name }) != -1 {
			assert.True(t, isStopped(&job))
		} else {
			assert.False(t, isStopped(&job))
		}
	}
}
