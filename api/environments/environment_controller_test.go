package environments

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	deploymentModels "github.com/statoil/radix-api/api/deployments/models"
	environmentModels "github.com/statoil/radix-api/api/environments/models"
	controllertest "github.com/statoil/radix-api/api/test"
	"github.com/statoil/radix-api/api/utils"
	commontest "github.com/statoil/radix-operator/pkg/apis/test"
	builders "github.com/statoil/radix-operator/pkg/apis/utils"
	k8sObjectUtils "github.com/statoil/radix-operator/pkg/apis/utils"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	"github.com/statoil/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

const (
	anyAppName = "any-app"
)

func setupTest() (*commontest.Utils, *controllertest.Utils, kubernetes.Interface, radixclient.Interface) {
	// Setup
	kubeclient := kubefake.NewSimpleClientset()
	radixclient := fake.NewSimpleClientset()

	// commonTestUtils is used for creating CRDs
	commonTestUtils := commontest.NewTestUtils(kubeclient, radixclient)

	// controllerTestUtils is used for issuing HTTP request and processing responses
	controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, NewEnvironmentController())

	return &commonTestUtils, &controllerTestUtils, kubeclient, radixclient
}

func TestGetEnvironmentDeployments_SortedWithFromTo(t *testing.T) {
	deploymentOneImage := "abcdef"
	deploymentTwoImage := "ghijkl"
	deploymentThreeImage := "mnopqr"
	layout := "2006-01-02T15:04:05.000Z"
	deploymentOneCreated, _ := time.Parse(layout, "2018-11-12T11:45:26.371Z")
	deploymentTwoCreated, _ := time.Parse(layout, "2018-11-12T12:30:14.000Z")
	deploymentThreeCreated, _ := time.Parse(layout, "2018-11-20T09:00:00.000Z")
	envName := "dev"

	// Setup
	commonTestUtils, controllerTestUtils, _, _ := setupTest()
	setupGetDeploymentsTest(commonTestUtils, anyAppName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated, envName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/deployments", anyAppName, envName))
	response := <-responseChannel

	deployments := make([]*deploymentModels.DeploymentSummary, 0)
	controllertest.GetResponseBody(response, &deployments)
	assert.Equal(t, 3, len(deployments))

	assert.Equal(t, deploymentThreeImage, deployments[0].Name)
	assert.Equal(t, utils.FormatTimestamp(deploymentThreeCreated), deployments[0].ActiveFrom)
	assert.Equal(t, "", deployments[0].ActiveTo)

	assert.Equal(t, deploymentTwoImage, deployments[1].Name)
	assert.Equal(t, utils.FormatTimestamp(deploymentTwoCreated), deployments[1].ActiveFrom)
	assert.Equal(t, utils.FormatTimestamp(deploymentThreeCreated), deployments[1].ActiveTo)

	assert.Equal(t, deploymentOneImage, deployments[2].Name)
	assert.Equal(t, utils.FormatTimestamp(deploymentOneCreated), deployments[2].ActiveFrom)
	assert.Equal(t, utils.FormatTimestamp(deploymentTwoCreated), deployments[2].ActiveTo)
}

func TestGetEnvironmentDeployments_Latest(t *testing.T) {
	deploymentOneImage := "abcdef"
	deploymentTwoImage := "ghijkl"
	deploymentThreeImage := "mnopqr"
	layout := "2006-01-02T15:04:05.000Z"
	deploymentOneCreated, _ := time.Parse(layout, "2018-11-12T11:45:26.371Z")
	deploymentTwoCreated, _ := time.Parse(layout, "2018-11-12T12:30:14.000Z")
	deploymentThreeCreated, _ := time.Parse(layout, "2018-11-20T09:00:00.000Z")
	envName := "dev"

	// Setup
	commonTestUtils, controllerTestUtils, _, _ := setupTest()
	setupGetDeploymentsTest(commonTestUtils, anyAppName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated, envName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/deployments?latest=true", anyAppName, envName))
	response := <-responseChannel

	deployments := make([]*deploymentModels.DeploymentSummary, 0)
	controllertest.GetResponseBody(response, &deployments)
	assert.Equal(t, 1, len(deployments))

	assert.Equal(t, deploymentThreeImage, deployments[0].Name)
	assert.Equal(t, utils.FormatTimestamp(deploymentThreeCreated), deployments[0].ActiveFrom)
	assert.Equal(t, "", deployments[0].ActiveTo)
}

func TestGetEnvironmentSummary_ApplicationWithNoDeployments_EnvironmentPending(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _ := setupTest()

	anyAppName := "any-app"
	commonTestUtils.ApplyApplication(builders.
		NewRadixApplicationBuilder().
		WithRadixRegistration(builders.ARadixRegistration()).
		WithAppName(anyAppName).
		WithEnvironment("dev", "master"))

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments", anyAppName))
	response := <-responseChannel
	environments := make([]*environmentModels.EnvironmentSummary, 0)
	controllertest.GetResponseBody(response, &environments)

	assert.Equal(t, 1, len(environments))
	assert.Equal(t, "dev", environments[0].Name)
	assert.Equal(t, environmentModels.Pending.String(), environments[0].Status)
	assert.Equal(t, "master", environments[0].BranchMapping)
	assert.Nil(t, environments[0].ActiveDeployment)
}

func TestGetEnvironmentSummary_ApplicationWithDeployment_EnvironmentConsistent(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _ := setupTest()

	anyAppName := "any-app"
	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithRadixApplication(builders.
			NewRadixApplicationBuilder().
			WithRadixRegistration(builders.ARadixRegistration()).
			WithAppName(anyAppName).
			WithEnvironment("dev", "master")).
		WithAppName(anyAppName).
		WithEnvironment("dev"))

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments", anyAppName))
	response := <-responseChannel
	environments := make([]*environmentModels.EnvironmentSummary, 0)
	controllertest.GetResponseBody(response, &environments)

	assert.Equal(t, environmentModels.Consistent.String(), environments[0].Status)
	assert.NotNil(t, environments[0].ActiveDeployment)
}

func TestGetEnvironmentSummary_RemoveEnvironmentFromConfig_OrphanedEnvironment(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _ := setupTest()

	anyAppName := "any-app"
	anyOrphanedEnvironment := "feature"

	commonTestUtils.ApplyRegistration(builders.
		NewRegistrationBuilder().
		WithName(anyAppName))

	commonTestUtils.ApplyApplication(builders.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment("dev", "master").
		WithEnvironment(anyOrphanedEnvironment, "feature"))

	commonTestUtils.ApplyDeployment(builders.
		NewDeploymentBuilder().
		WithAppName(anyAppName).
		WithEnvironment("dev").
		WithImageTag("someimageindev"))

	commonTestUtils.ApplyDeployment(builders.
		NewDeploymentBuilder().
		WithAppName(anyAppName).
		WithEnvironment(anyOrphanedEnvironment).
		WithImageTag("someimageinfeature"))

	// Remove feature environment from application config
	commonTestUtils.ApplyApplicationUpdate(builders.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment("dev", "master"))

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments", anyAppName))
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

func TestGetEnvironment_NoExistingEnvironment_ReturnsAnError(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _ := setupTest()

	anyAppName := "any-app"

	commonTestUtils.ApplyApplication(builders.
		ARadixApplication().
		WithAppName(anyAppName).
		WithEnvironment("dev", "master"))

	// Test
	anyNonExistingEnvironment := "non-existing-environment"
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyNonExistingEnvironment))
	response := <-responseChannel

	assert.Equal(t, http.StatusNotFound, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	expectedError := environmentModels.NonExistingEnvironment(nil, anyAppName, anyNonExistingEnvironment)
	assert.Equal(t, (expectedError.(*utils.Error)).Message, errorResponse.Message)

}

func TestGetEnvironment_ExistingEnvironmentInConfig_ReturnsAPendingEnvironment(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _ := setupTest()

	anyAppName := "any-app"

	commonTestUtils.ApplyApplication(builders.
		ARadixApplication().
		WithAppName(anyAppName).
		WithEnvironment("dev", "master"))

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, "dev"))
	response := <-responseChannel

	assert.Equal(t, http.StatusOK, response.Code)

	environment := environmentModels.Environment{}
	controllertest.GetResponseBody(response, &environment)
	assert.Equal(t, "dev", environment.Name)
	assert.Equal(t, environmentModels.Pending.String(), environment.Status)
}

func TestGetEnvironment_RemoveEnvironmentFromConfig_OrphanedEnvironment(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _ := setupTest()

	anyAppName := "any-app"
	anyOrphanedEnvironment := "feature"

	commonTestUtils.ApplyRegistration(builders.
		NewRegistrationBuilder().
		WithName(anyAppName))

	commonTestUtils.ApplyApplication(builders.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment("dev", "master").
		WithEnvironment(anyOrphanedEnvironment, "feature"))

	commonTestUtils.ApplyDeployment(builders.
		NewDeploymentBuilder().
		WithAppName(anyAppName).
		WithEnvironment("dev").
		WithImageTag("someimageindev"))

	commonTestUtils.ApplyDeployment(builders.
		NewDeploymentBuilder().
		WithAppName(anyAppName).
		WithEnvironment(anyOrphanedEnvironment).
		WithImageTag("someimageinfeature"))

	// Remove feature environment from application config
	commonTestUtils.ApplyApplicationUpdate(builders.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment("dev", "master"))

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyOrphanedEnvironment))
	response := <-responseChannel

	assert.Equal(t, http.StatusOK, response.Code)

	environment := environmentModels.Environment{}
	controllertest.GetResponseBody(response, &environment)
	assert.Equal(t, anyOrphanedEnvironment, environment.Name)
	assert.Equal(t, environmentModels.Orphan.String(), environment.Status)
	assert.NotNil(t, environment.ActiveDeployment)
	assert.Equal(t, 1, len(environment.Deployments))

	// Currently we don't support branch mapping for orphaned environments
	assert.Equal(t, "", environment.BranchMapping)

}

func setupGetDeploymentsTest(commonTestUtils *commontest.Utils, appName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage string, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated time.Time, environment string) {
	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithDeploymentName(deploymentOneImage).
		WithAppName(appName).
		WithEnvironment(environment).
		WithImageTag(deploymentOneImage).
		WithCreated(deploymentOneCreated))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithDeploymentName(deploymentTwoImage).
		WithAppName(appName).
		WithEnvironment(environment).
		WithImageTag(deploymentTwoImage).
		WithCreated(deploymentTwoCreated))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithDeploymentName(deploymentThreeImage).
		WithAppName(appName).
		WithEnvironment(environment).
		WithImageTag(deploymentThreeImage).
		WithCreated(deploymentThreeCreated))
}

func executeUpdateSecretTest(appName, existingEnvName, requestEnvName, existingComponentName, requestComponentName, oldSecretName, oldSecretValue, updateSecretName, updateSecretValue string) *httptest.ResponseRecorder {
	parameters := environmentModels.SecretParameters{
		SecretValue: updateSecretValue,
	}

	_, controllerTestUtils, kubeclient, _ := setupTest()

	ns := k8sObjectUtils.GetEnvironmentNamespace(appName, existingEnvName)

	namespace := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}
	kubeclient.CoreV1().Namespaces().Create(&namespace)

	secretObject := v1.Secret{
		Type: "Opaque",
		ObjectMeta: metav1.ObjectMeta{
			Name: existingComponentName,
		},
		Data: map[string][]byte{oldSecretName: []byte(oldSecretValue)},
	}
	kubeclient.CoreV1().Secrets(ns).Create(&secretObject)

	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/secrets/%s", appName, requestEnvName, requestComponentName, updateSecretName), parameters)
	response := <-responseChannel
	return response
}

func TestUpdateSecret_OK(t *testing.T) {
	appName := "test-app"
	existingEnvName := "dev"
	requestEnvName := "dev"
	existingComponentName := "backend"
	requestComponentName := "backend"
	oldSecretName := "TEST_SECRET"
	oldSecretValue := "oldvalue"
	updateSecretName := "TEST_SECRET"
	updateSecretValue := "newvalue"

	response := executeUpdateSecretTest(appName, existingEnvName, requestEnvName, existingComponentName, requestComponentName, oldSecretName, oldSecretValue, updateSecretName, updateSecretValue)
	assert.Equal(t, http.StatusOK, response.Code)
}

func TestUpdateSecret_SecretName_Missing(t *testing.T) {
	appName := "test-app"
	existingEnvName := "dev"
	requestEnvName := "dev"
	existingComponentName := "backend"
	requestComponentName := "backend"
	oldSecretName := "TEST"
	oldSecretValue := "oldvalue"
	updateSecretName := "TEST_SECRET"
	updateSecretValue := "newvalue"

	response := executeUpdateSecretTest(appName, existingEnvName, requestEnvName, existingComponentName, requestComponentName, oldSecretName, oldSecretValue, updateSecretName, updateSecretValue)
	errorResponse, _ := controllertest.GetErrorResponse(response)

	assert.Equal(t, http.StatusUnprocessableEntity, response.Code)
	assert.Equal(t, "Secret name does not exist", errorResponse.Message)
	assert.Equal(t, "Secret failed validation", errorResponse.Err.Error())
}

func TestUpdateSecret_SecretValue_Empty(t *testing.T) {
	appName := "test-app"
	existingEnvName := "dev"
	requestEnvName := "dev"
	existingComponentName := "backend"
	requestComponentName := "backend"
	oldSecretName := "TEST_SECRET"
	oldSecretValue := "oldvalue"
	updateSecretName := "TEST_SECRET"
	updateSecretValue := ""

	response := executeUpdateSecretTest(appName, existingEnvName, requestEnvName, existingComponentName, requestComponentName, oldSecretName, oldSecretValue, updateSecretName, updateSecretValue)
	errorResponse, _ := controllertest.GetErrorResponse(response)

	assert.Equal(t, http.StatusUnprocessableEntity, response.Code)
	assert.Equal(t, "New secret value is empty", errorResponse.Message)
	assert.Equal(t, "Secret failed validation", errorResponse.Err.Error())
}

func TestUpdateSecret_SecretValue_NoChange(t *testing.T) {
	appName := "test-app"
	existingEnvName := "dev"
	requestEnvName := "dev"
	existingComponentName := "backend"
	requestComponentName := "backend"
	oldSecretName := "TEST_SECRET"
	oldSecretValue := "oldvalue"
	updateSecretName := "TEST_SECRET"
	updateSecretValue := "oldvalue"

	response := executeUpdateSecretTest(appName, existingEnvName, requestEnvName, existingComponentName, requestComponentName, oldSecretName, oldSecretValue, updateSecretName, updateSecretValue)
	errorResponse, _ := controllertest.GetErrorResponse(response)

	assert.Equal(t, http.StatusUnprocessableEntity, response.Code)
	assert.Equal(t, "No change in secret value", errorResponse.Message)
	assert.Equal(t, "Secret failed validation", errorResponse.Err.Error())
}

func TestUpdateSecret_SecretObject_Missing(t *testing.T) {
	appName := "test-app"
	existingEnvName := "dev"
	requestEnvName := "dev"
	existingComponentName := "backend"
	requestComponentName := "frontend"
	oldSecretName := "TEST_SECRET"
	oldSecretValue := "oldvalue"
	updateSecretName := "TEST_SECRET"
	updateSecretValue := "newvalue"

	response := executeUpdateSecretTest(appName, existingEnvName, requestEnvName, existingComponentName, requestComponentName, oldSecretName, oldSecretValue, updateSecretName, updateSecretValue)
	errorResponse, _ := controllertest.GetErrorResponse(response)

	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Equal(t, "Secret object does not exist", errorResponse.Message)
	assert.Equal(t, "secrets \"frontend\" not found", errorResponse.Err.Error())
}

func TestUpdateSecret_Namespace_Missing(t *testing.T) {
	appName := "test-app"
	existingEnvName := "dev"
	requestEnvName := "prod"
	existingComponentName := "backend"
	requestComponentName := "backend"
	oldSecretName := "TEST_SECRET"
	oldSecretValue := "oldvalue"
	updateSecretName := "TEST_SECRET"
	updateSecretValue := "newvalue"

	response := executeUpdateSecretTest(appName, existingEnvName, requestEnvName, existingComponentName, requestComponentName, oldSecretName, oldSecretValue, updateSecretName, updateSecretValue)
	errorResponse, _ := controllertest.GetErrorResponse(response)

	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Equal(t, "Secret object does not exist", errorResponse.Message)
	assert.Equal(t, "secrets \"backend\" not found", errorResponse.Err.Error())
}
