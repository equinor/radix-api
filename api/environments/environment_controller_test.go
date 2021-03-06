package environments

import (
	"context"
	"fmt"
	radixhttp "github.com/equinor/radix-common/net/http"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	environmentModels "github.com/equinor/radix-api/api/environments/models"
	event "github.com/equinor/radix-api/api/events"
	eventMock "github.com/equinor/radix-api/api/events/mock"
	eventModels "github.com/equinor/radix-api/api/events/models"
	"github.com/equinor/radix-api/api/test"
	controllertest "github.com/equinor/radix-api/api/test"
	"github.com/equinor/radix-api/api/utils"
	"github.com/equinor/radix-api/models"
	radixmodels "github.com/equinor/radix-common/models"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	builders "github.com/equinor/radix-operator/pkg/apis/utils"
	k8sObjectUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	prometheusclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/assert"
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
)

func setupTest() (*commontest.Utils, *controllertest.Utils, kubernetes.Interface, radixclient.Interface, prometheusclient.Interface) {
	// Setup
	kubeclient := kubefake.NewSimpleClientset()
	radixclient := fake.NewSimpleClientset()
	prometheusclient := prometheusfake.NewSimpleClientset()

	// commonTestUtils is used for creating CRDs
	commonTestUtils := commontest.NewTestUtils(kubeclient, radixclient)
	commonTestUtils.CreateClusterPrerequisites(clusterName, containerRegistry)

	// controllerTestUtils is used for issuing HTTP request and processing responses
	controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, NewEnvironmentController())

	return &commonTestUtils, &controllerTestUtils, kubeclient, radixclient, prometheusclient
}

func TestUpdateSecret_TLSSecretForExternalAlias_UpdatedOk(t *testing.T) {
	anyComponent := "frontend"

	// Setup
	commonTestUtils, controllerTestUtils, client, radixclient, promclient := setupTest()
	utils.ApplyDeploymentWithSync(client, radixclient, promclient, commonTestUtils,
		builders.ARadixDeployment().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithRadixApplication(builders.ARadixApplication().
				WithAppName(anyAppName).
				WithEnvironment(anyEnvironment, "master").
				WithDNSExternalAlias("some.alias.com", anyEnvironment, anyComponent).
				WithDNSExternalAlias("another.alias.com", anyEnvironment, anyComponent)).
			WithComponents(
				builders.NewDeployComponentBuilder().
					WithName(anyComponent).
					WithPort("http", 8080).
					WithPublicPort("http").
					WithDNSExternalAlias("some.alias.com").
					WithDNSExternalAlias("another.alias.com")))

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyEnvironment))
	response := <-responseChannel

	environment := environmentModels.Environment{}
	controllertest.GetResponseBody(response, &environment)
	assert.Equal(t, 4, len(environment.Secrets))
	assert.True(t, contains(environment.Secrets, "some.alias.com-cert"))
	assert.True(t, contains(environment.Secrets, "some.alias.com-key"))
	assert.True(t, contains(environment.Secrets, "another.alias.com-cert"))
	assert.True(t, contains(environment.Secrets, "another.alias.com-key"))

	parameters := environmentModels.SecretParameters{
		SecretValue: "anyValue",
	}

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/secrets/%s", anyAppName, anyEnvironment, anyComponentName, environment.Secrets[0].Name), parameters)
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/secrets/%s", anyAppName, anyEnvironment, anyComponentName, environment.Secrets[1].Name), parameters)
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
}

func TestUpdateSecret_AccountSecretForComponentVolumeMount_UpdatedOk(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, client, radixclient, promclient := setupTest()
	utils.ApplyDeploymentWithSync(client, radixclient, promclient, commonTestUtils,
		builders.ARadixDeployment().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithRadixApplication(builders.ARadixApplication().
				WithAppName(anyAppName).
				WithEnvironment(anyEnvironment, "master")).
			WithComponents(
				builders.NewDeployComponentBuilder().
					WithName(anyComponentName).
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
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyEnvironment))
	response := <-responseChannel

	environment := environmentModels.Environment{}
	controllertest.GetResponseBody(response, &environment)
	assert.Equal(t, 2, len(environment.Secrets))
	assert.True(t, contains(environment.Secrets, fmt.Sprintf("%v-somevolumename-blobfusecreds-accountkey", anyComponentName)))
	assert.True(t, contains(environment.Secrets, fmt.Sprintf("%v-somevolumename-blobfusecreds-accountname", anyComponentName)))

	parameters := environmentModels.SecretParameters{
		SecretValue: "anyValue",
	}

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/secrets/%s", anyAppName, anyEnvironment, anyComponentName, environment.Secrets[0].Name), parameters)
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
}

func TestUpdateSecret_AccountSecretForJobVolumeMount_UpdatedOk(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, client, radixclient, promclient := setupTest()
	utils.ApplyDeploymentWithSync(client, radixclient, promclient, commonTestUtils,
		builders.ARadixDeployment().
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithRadixApplication(builders.ARadixApplication().
				WithAppName(anyAppName).
				WithEnvironment(anyEnvironment, "master")).
			WithJobComponents(
				builders.NewDeployJobComponentBuilder().
					WithName(anyJobName).
					WithVolumeMounts([]v1.RadixVolumeMount{
						{
							Type:      v1.MountTypeBlob,
							Name:      "somevolumename",
							Container: "some-container",
							Path:      "some-path",
						},
					})))

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyEnvironment))
	response := <-responseChannel

	environment := environmentModels.Environment{}
	controllertest.GetResponseBody(response, &environment)
	assert.Equal(t, 2, len(environment.Secrets))
	assert.True(t, contains(environment.Secrets, fmt.Sprintf("%v-somevolumename-blobfusecreds-accountkey", anyJobName)))
	assert.True(t, contains(environment.Secrets, fmt.Sprintf("%v-somevolumename-blobfusecreds-accountname", anyJobName)))

	parameters := environmentModels.SecretParameters{
		SecretValue: "anyValue",
	}

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/secrets/%s", anyAppName, anyEnvironment, anyJobName, environment.Secrets[0].Name), parameters)
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
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
	commonTestUtils, controllerTestUtils, _, _, _ := setupTest()
	setupGetDeploymentsTest(commonTestUtils, anyAppName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated, envName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/deployments", anyAppName, envName))
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
	envName := "dev"

	// Setup
	commonTestUtils, controllerTestUtils, _, _, _ := setupTest()
	setupGetDeploymentsTest(commonTestUtils, anyAppName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated, envName)

	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/deployments?latest=true", anyAppName, envName))
	response := <-responseChannel

	deployments := make([]*deploymentModels.DeploymentSummary, 0)
	controllertest.GetResponseBody(response, &deployments)
	assert.Equal(t, 1, len(deployments))

	assert.Equal(t, deploymentThreeImage, deployments[0].Name)
	assert.Equal(t, radixutils.FormatTimestamp(deploymentThreeCreated), deployments[0].ActiveFrom)
	assert.Equal(t, "", deployments[0].ActiveTo)
}

func TestGetEnvironmentSummary_ApplicationWithNoDeployments_EnvironmentPending(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _, _ := setupTest()

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
	commonTestUtils, controllerTestUtils, _, _, _ := setupTest()

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
	commonTestUtils, controllerTestUtils, _, _, _ := setupTest()

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

func TestGetEnvironmentSummary_OrphanedEnvironmentWithDash_OrphanedEnvironmentIsListedOk(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _, _ := setupTest()

	anyAppName := "any-app"
	anyOrphanedEnvironment := "feature-1"

	rr, _ := commonTestUtils.ApplyRegistration(builders.
		NewRegistrationBuilder().
		WithName(anyAppName))

	commonTestUtils.ApplyApplication(builders.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment("dev", "master"))

	commonTestUtils.ApplyEnvironment(builders.
		NewEnvironmentBuilder().
		WithAppLabel().
		WithAppName(anyAppName).
		WithEnvironmentName(anyOrphanedEnvironment).
		WithRegistrationOwner(rr).
		WithOrphaned(true))

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments", anyAppName))
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
	// Setup
	commonTestUtils, controllerTestUtils, _, _, _ := setupTest()

	anyAppName := "any-app"
	anyNonOrphanedEnvironment := "dev"
	anyOrphanedEnvironment := "feature"

	commonTestUtils.ApplyApplication(builders.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment(anyNonOrphanedEnvironment, "master").
		WithRadixRegistration(builders.
			NewRegistrationBuilder().
			WithName(anyAppName)))

	commonTestUtils.ApplyEnvironment(builders.
		NewEnvironmentBuilder().
		WithAppLabel().
		WithAppName(anyAppName).
		WithEnvironmentName(anyOrphanedEnvironment))

	// Test
	// Start with two environments
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments", anyAppName))
	response := <-responseChannel
	environments := make([]*environmentModels.EnvironmentSummary, 0)
	controllertest.GetResponseBody(response, &environments)
	assert.Equal(t, 2, len(environments))

	// Orphaned environment can be deleted
	responseChannel = controllerTestUtils.ExecuteRequest("DELETE", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyOrphanedEnvironment))
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)

	// Non-orphaned cannot
	responseChannel = controllerTestUtils.ExecuteRequest("DELETE", fmt.Sprintf("/api/v1/applications/%s/environments/%s", anyAppName, anyNonOrphanedEnvironment))
	response = <-responseChannel
	assert.Equal(t, http.StatusBadRequest, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	expectedError := environmentModels.CannotDeleteNonOrphanedEnvironment(anyAppName, anyNonOrphanedEnvironment)
	assert.Equal(t, (expectedError.(*radixhttp.Error)).Message, errorResponse.Message)

	// Only one remaining environment after delete
	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments", anyAppName))
	response = <-responseChannel
	environments = make([]*environmentModels.EnvironmentSummary, 0)
	controllertest.GetResponseBody(response, &environments)
	assert.Equal(t, 1, len(environments))

}

func TestGetEnvironment_NoExistingEnvironment_ReturnsAnError(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _, _ := setupTest()

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
	assert.Equal(t, (expectedError.(*radixhttp.Error)).Message, errorResponse.Message)

}

func TestGetEnvironment_ExistingEnvironmentInConfig_ReturnsAPendingEnvironment(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _, _ := setupTest()

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
	err := controllertest.GetResponseBody(response, &environment)
	assert.Nil(t, err)
	assert.Equal(t, "dev", environment.Name)
	assert.Equal(t, environmentModels.Pending.String(), environment.Status)
}

func setupGetDeploymentsTest(commonTestUtils *commontest.Utils, appName, deploymentOneImage, deploymentTwoImage, deploymentThreeImage string, deploymentOneCreated, deploymentTwoCreated, deploymentThreeCreated time.Time, environment string) {
	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithDeploymentName(deploymentOneImage).
		WithAppName(appName).
		WithEnvironment(environment).
		WithImageTag(deploymentOneImage).
		WithCreated(deploymentOneCreated).
		WithCondition(v1.DeploymentInactive).
		WithActiveFrom(deploymentOneCreated).
		WithActiveTo(deploymentTwoCreated))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithDeploymentName(deploymentTwoImage).
		WithAppName(appName).
		WithEnvironment(environment).
		WithImageTag(deploymentTwoImage).
		WithCreated(deploymentTwoCreated).
		WithCondition(v1.DeploymentInactive).
		WithActiveFrom(deploymentTwoCreated).
		WithActiveTo(deploymentThreeCreated))

	commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithDeploymentName(deploymentThreeImage).
		WithAppName(appName).
		WithEnvironment(environment).
		WithImageTag(deploymentThreeImage).
		WithCreated(deploymentThreeCreated).
		WithCondition(v1.DeploymentActive).
		WithActiveFrom(deploymentThreeCreated))
}

func executeUpdateComponentSecretTest(oldSecretValue, updateEnvironment, updateComponent, updateSecretName, updateSecretValue string) *httptest.ResponseRecorder {
	response := executeUpdateSecretTest(
		oldSecretValue,
		updateEnvironment,
		updateComponent,
		updateSecretName,
		updateSecretValue,
		configureApplicationComponentSecret)

	return response
}

func executeUpdateJobSecretTest(oldSecretValue, updateEnvironment, updateComponent, updateSecretName, updateSecretValue string) *httptest.ResponseRecorder {
	response := executeUpdateSecretTest(
		oldSecretValue,
		updateEnvironment,
		updateComponent,
		updateSecretName,
		updateSecretValue,
		configureApplicationJobSecret)

	return response
}

func configureApplicationComponentSecret(builder *k8sObjectUtils.ApplicationBuilder) {
	(*builder).WithComponents(
		builders.AnApplicationComponent().
			WithName(anyComponentName).
			WithSecrets(anySecretName),
	)
}

func configureApplicationJobSecret(builder *k8sObjectUtils.ApplicationBuilder) {
	(*builder).WithJobComponents(
		builders.AnApplicationJobComponent().
			WithName(anyJobName).
			WithSecrets(anySecretName),
	)
}

func executeUpdateSecretTest(oldSecretValue, updateEnvironment, updateComponent, updateSecretName, updateSecretValue string, appConfigurator func(builder *k8sObjectUtils.ApplicationBuilder)) *httptest.ResponseRecorder {

	// Setup
	parameters := environmentModels.SecretParameters{
		SecretValue: updateSecretValue,
	}

	commonTestUtils, controllerTestUtils, kubeclient, _, _ := setupTest()
	appBuilder := builders.
		ARadixApplication().
		WithAppName(anyAppName)
	appConfigurator(&appBuilder)

	commonTestUtils.ApplyApplication(appBuilder)
	ns := k8sObjectUtils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}
	kubeclient.CoreV1().Namespaces().Create(context.TODO(), &namespace, metav1.CreateOptions{})

	// Component secret
	secretObject := corev1.Secret{
		Type: "Opaque",
		ObjectMeta: metav1.ObjectMeta{
			Name: k8sObjectUtils.GetComponentSecretName(anyComponentName),
		},
		Data: map[string][]byte{anySecretName: []byte(oldSecretValue)},
	}
	kubeclient.CoreV1().Secrets(ns).Create(context.TODO(), &secretObject, metav1.CreateOptions{})

	// Job secret
	secretObject = corev1.Secret{
		Type: "Opaque",
		ObjectMeta: metav1.ObjectMeta{
			Name: k8sObjectUtils.GetComponentSecretName(anyJobName),
		},
		Data: map[string][]byte{anySecretName: []byte(oldSecretValue)},
	}
	kubeclient.CoreV1().Secrets(ns).Create(context.TODO(), &secretObject, metav1.CreateOptions{})

	// Test
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/secrets/%s", anyAppName, updateEnvironment, updateComponent, updateSecretName), parameters)
	response := <-responseChannel
	return response
}

func TestUpdateSecret_OK(t *testing.T) {
	oldSecretValue := "oldvalue"
	updateSecretValue := "newvalue"

	response := executeUpdateComponentSecretTest(oldSecretValue, anyEnvironment, anyComponentName, anySecretName, updateSecretValue)
	assert.Equal(t, http.StatusOK, response.Code)

	response = executeUpdateJobSecretTest(oldSecretValue, anyEnvironment, anyJobName, anySecretName, updateSecretValue)
	assert.Equal(t, http.StatusOK, response.Code)
}

func TestUpdateSecret_NonExistingSecret_Missing(t *testing.T) {
	nonExistingSecretName := "TEST"
	oldSecretValue := "oldvalue"
	updateSecretValue := "newvalue"

	response := executeUpdateComponentSecretTest(oldSecretValue, anyEnvironment, anyComponentName, nonExistingSecretName, updateSecretValue)
	assert.Equal(t, http.StatusOK, response.Code)

	response = executeUpdateJobSecretTest(oldSecretValue, anyEnvironment, anyJobName, nonExistingSecretName, updateSecretValue)
	assert.Equal(t, http.StatusOK, response.Code)
}

func TestUpdateSecret_EmptySecretValue_ValidationError(t *testing.T) {
	oldSecretValue := "oldvalue"
	updateSecretValue := ""

	response := executeUpdateComponentSecretTest(oldSecretValue, anyEnvironment, anyComponentName, anySecretName, updateSecretValue)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Equal(t, "New secret value is empty", errorResponse.Message)
	assert.Equal(t, "Secret failed validation", errorResponse.Err.Error())

	response = executeUpdateJobSecretTest(oldSecretValue, anyEnvironment, anyJobName, anySecretName, updateSecretValue)
	errorResponse, _ = controllertest.GetErrorResponse(response)
	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Equal(t, "New secret value is empty", errorResponse.Message)
	assert.Equal(t, "Secret failed validation", errorResponse.Err.Error())
}

func TestUpdateSecret_NoUpdate_NoError(t *testing.T) {
	oldSecretValue := "oldvalue"
	updateSecretValue := "oldvalue"

	response := executeUpdateComponentSecretTest(oldSecretValue, anyEnvironment, anyComponentName, anySecretName, updateSecretValue)
	assert.Equal(t, http.StatusOK, response.Code)

	response = executeUpdateJobSecretTest(oldSecretValue, anyEnvironment, anyJobName, anySecretName, updateSecretValue)
	assert.Equal(t, http.StatusOK, response.Code)
}

func TestUpdateSecret_NonExistingComponent_Missing(t *testing.T) {
	nonExistingComponent := "frontend"
	nonExistingSecretObjName := k8sObjectUtils.GetComponentSecretName(nonExistingComponent)
	oldSecretValue := "oldvalue"
	updateSecretValue := "newvalue"

	response := executeUpdateComponentSecretTest(oldSecretValue, anyEnvironment, nonExistingComponent, anySecretName, updateSecretValue)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Equal(t, "Secret object does not exist", errorResponse.Message)
	assert.Equal(t, fmt.Sprintf("secrets \"%s\" not found", nonExistingSecretObjName), errorResponse.Err.Error())

	response = executeUpdateJobSecretTest(oldSecretValue, anyEnvironment, nonExistingComponent, anySecretName, updateSecretValue)
	errorResponse, _ = controllertest.GetErrorResponse(response)
	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Equal(t, "Secret object does not exist", errorResponse.Message)
	assert.Equal(t, fmt.Sprintf("secrets \"%s\" not found", nonExistingSecretObjName), errorResponse.Err.Error())
}

func TestUpdateSecret_NonExistingEnvironment_Missing(t *testing.T) {
	nonExistingEnvironment := "prod"
	oldSecretValue := "oldvalue"
	updateSecretValue := "newvalue"

	response := executeUpdateComponentSecretTest(oldSecretValue, nonExistingEnvironment, anyComponentName, anySecretName, updateSecretValue)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	secretObjName := k8sObjectUtils.GetComponentSecretName(anyComponentName)
	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Equal(t, "Secret object does not exist", errorResponse.Message)
	assert.Equal(t, fmt.Sprintf("secrets \"%s\" not found", secretObjName), errorResponse.Err.Error())

	response = executeUpdateJobSecretTest(oldSecretValue, nonExistingEnvironment, anyJobName, anySecretName, updateSecretValue)
	errorResponse, _ = controllertest.GetErrorResponse(response)
	secretObjName = k8sObjectUtils.GetComponentSecretName(anyJobName)
	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Equal(t, "Secret object does not exist", errorResponse.Message)
	assert.Equal(t, fmt.Sprintf("secrets \"%s\" not found", secretObjName), errorResponse.Err.Error())
}

func componentBuilderFromSecretMap(secretsMap map[string][]string) func(*k8sObjectUtils.DeploymentBuilder) {
	return func(deployBuilder *k8sObjectUtils.DeploymentBuilder) {
		componentBuilders := make([]builders.DeployComponentBuilder, 0, len(secretsMap))
		for componentName, componentSecrets := range secretsMap {
			component := builders.
				NewDeployComponentBuilder().
				WithName(componentName).
				WithSecrets(componentSecrets)
			componentBuilders = append(componentBuilders, component)
		}
		(*deployBuilder).WithComponents(componentBuilders...)
	}
}

func jobBuilderFromSecretMap(secretsMap map[string][]string) func(*k8sObjectUtils.DeploymentBuilder) {
	return func(deployBuilder *k8sObjectUtils.DeploymentBuilder) {
		jobBuilders := make([]builders.DeployJobComponentBuilder, 0, len(secretsMap))
		for jobName, jobSecret := range secretsMap {
			job := builders.
				NewDeployJobComponentBuilder().
				WithName(jobName).
				WithSecrets(jobSecret)
			jobBuilders = append(jobBuilders, job)
		}
		(*deployBuilder).WithJobComponents(jobBuilders...)
	}
}

func applyTestEnvironmentComponentSecrets(commonTestUtils *commontest.Utils, kubeclient kubernetes.Interface, appName, environmentName, buildFrom string, componentSecretsMap map[string][]string, clusterComponentSecretsMap map[string]map[string][]byte) {
	configurator := componentBuilderFromSecretMap(componentSecretsMap)
	applyTestEnvironmentSecrets(commonTestUtils, kubeclient, appName, environmentName, buildFrom, clusterComponentSecretsMap, configurator)
}

func applyTestEnvironmentJobSecrets(commonTestUtils *commontest.Utils, kubeclient kubernetes.Interface, appName, environmentName, buildFrom string, componentSecretsMap map[string][]string, clusterComponentSecretsMap map[string]map[string][]byte) {
	configurator := jobBuilderFromSecretMap(componentSecretsMap)
	applyTestEnvironmentSecrets(commonTestUtils, kubeclient, appName, environmentName, buildFrom, clusterComponentSecretsMap, configurator)
}

func applyTestEnvironmentSecrets(commonTestUtils *commontest.Utils, kubeclient kubernetes.Interface, appName, environmentName, buildFrom string, clusterComponentSecretsMap map[string]map[string][]byte, deploymentConfigurator func(*k8sObjectUtils.DeploymentBuilder)) {
	ns := k8sObjectUtils.GetEnvironmentNamespace(appName, environmentName)

	deployBuilder := builders.
		NewDeploymentBuilder().
		WithRadixApplication(builders.ARadixApplication()).
		WithAppName(anyAppName).
		WithEnvironment(environmentName)

	deploymentConfigurator(&deployBuilder)

	commonTestUtils.ApplyDeployment(deployBuilder)

	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}
	kubeclient.CoreV1().Namespaces().Create(context.TODO(), &namespace, metav1.CreateOptions{})

	for componentName, clusterComponentSecrets := range clusterComponentSecretsMap {
		secretObject := corev1.Secret{
			Type: "Opaque",
			ObjectMeta: metav1.ObjectMeta{
				Name: k8sObjectUtils.GetComponentSecretName(componentName),
			},
			Data: clusterComponentSecrets,
		}
		kubeclient.CoreV1().Secrets(ns).Create(context.TODO(), &secretObject, metav1.CreateOptions{})
	}
}

func assertSecretObject(t *testing.T, secretObject environmentModels.Secret, name, component, status, testname string) {
	assert.Equal(t, name, secretObject.Name, testname, fmt.Sprintf("%s: incorrect secret name", testname))
	assert.Equal(t, component, secretObject.Component, fmt.Sprintf("%s: incorrect component name", testname))
	assert.Equal(t, status, secretObject.Status, fmt.Sprintf("%s: incorrect secret status", testname))
}

type secretTestFunc func(commonTestUtils *commontest.Utils, kubeclient kubernetes.Interface, appName, environmentName, buildFrom string, componentSecretsMap map[string][]string, clusterComponentSecretsMap map[string]map[string][]byte)

type secretTestDefinition struct {
	name   string
	tester secretTestFunc
}

var secretTestFunctions []secretTestDefinition = []secretTestDefinition{
	{name: "component secrets", tester: applyTestEnvironmentComponentSecrets},
	{name: "job secrets", tester: applyTestEnvironmentJobSecrets},
}

func TestGetEnvironmentSecrets_OneComponent_AllConsistent(t *testing.T) {
	for _, test := range secretTestFunctions {
		commonTestUtils, _, kubeclient, radixclient, _ := setupTest()
		handler := initHandler(kubeclient, radixclient)

		appName := "any-app"
		componentOneName := "backend"
		environmentOne := "dev"
		buildFrom := "master"
		secretA := "a"
		secretB := "b"
		secretC := "c"

		componentSecrets := []string{secretA, secretB, secretC}
		componentSecretsMap := map[string][]string{
			componentOneName: componentSecrets,
		}

		clusterComponentSecrets := map[string][]byte{
			secretA: []byte(secretA),
			secretB: []byte(secretB),
			secretC: []byte(secretC),
		}
		clusterComponentSecretsMap := map[string]map[string][]byte{
			componentOneName: clusterComponentSecrets,
		}

		test.tester(commonTestUtils, kubeclient, appName, environmentOne, buildFrom, componentSecretsMap, clusterComponentSecretsMap)

		secrets, _ := handler.GetEnvironmentSecrets(appName, environmentOne)

		assert.Equal(t, 3, len(secrets), fmt.Sprintf("%s: incorrect secret count", test.name))
		for _, aSecret := range secrets {
			if aSecret.Name == secretA {
				assertSecretObject(t, aSecret, secretA, componentOneName, "Consistent", test.name)
			}
			if aSecret.Name == secretB {
				assertSecretObject(t, aSecret, secretB, componentOneName, "Consistent", test.name)
			}
			if aSecret.Name == secretC {
				assertSecretObject(t, aSecret, secretC, componentOneName, "Consistent", test.name)
			}
		}
	}
}

func TestGetEnvironmentSecrets_OneComponent_PartiallyConsistent(t *testing.T) {
	for _, test := range secretTestFunctions {
		commonTestUtils, _, kubeclient, radixclient, _ := setupTest()
		handler := initHandler(kubeclient, radixclient)

		appName := "any-app"
		componentOneName := "backend"
		environmentOne := "dev"
		buildFrom := "master"
		secretA := "a"
		secretB := "b"
		secretC := "c"
		secretD := "d"

		componentSecrets := []string{secretA, secretB, secretC}
		componentSecretsMap := map[string][]string{
			componentOneName: componentSecrets,
		}

		clusterComponentSecrets := map[string][]byte{
			secretB: []byte(secretB),
			secretC: []byte(secretC),
			secretD: []byte(secretD),
		}
		clusterComponentSecretsMap := map[string]map[string][]byte{
			componentOneName: clusterComponentSecrets,
		}

		test.tester(commonTestUtils, kubeclient, appName, environmentOne, buildFrom, componentSecretsMap, clusterComponentSecretsMap)

		secrets, _ := handler.GetEnvironmentSecrets(appName, environmentOne)

		assert.Equal(t, 4, len(secrets), fmt.Sprintf("%s: incorrect secret count", test.name))
		for _, aSecret := range secrets {
			if aSecret.Name == secretA {
				assertSecretObject(t, aSecret, secretA, componentOneName, "Pending", test.name)
			}
			if aSecret.Name == secretB {
				assertSecretObject(t, aSecret, secretB, componentOneName, "Consistent", test.name)
			}
			if aSecret.Name == secretC {
				assertSecretObject(t, aSecret, secretC, componentOneName, "Consistent", test.name)
			}
			if aSecret.Name == secretD {
				assertSecretObject(t, aSecret, secretD, componentOneName, "Orphan", test.name)
			}
		}
	}
}

func TestGetEnvironmentSecrets_OneComponent_NoConsistent(t *testing.T) {
	for _, test := range secretTestFunctions {
		commonTestUtils, _, kubeclient, radixclient, _ := setupTest()
		handler := initHandler(kubeclient, radixclient)

		appName := "any-app"
		componentOneName := "backend"
		environmentOne := "dev"
		buildFrom := "master"
		secretA := "a"
		secretB := "b"
		secretC := "c"
		secretD := "d"
		secretE := "e"
		secretF := "f"

		componentSecrets := []string{secretA, secretB, secretC}
		componentSecretsMap := map[string][]string{
			componentOneName: componentSecrets,
		}

		clusterComponentSecrets := map[string][]byte{
			secretD: []byte(secretD),
			secretE: []byte(secretE),
			secretF: []byte(secretF),
		}
		clusterComponentSecretsMap := map[string]map[string][]byte{
			componentOneName: clusterComponentSecrets,
		}

		test.tester(commonTestUtils, kubeclient, appName, environmentOne, buildFrom, componentSecretsMap, clusterComponentSecretsMap)

		secrets, _ := handler.GetEnvironmentSecrets(appName, environmentOne)

		assert.Equal(t, 6, len(secrets), fmt.Sprintf("%s: incorrect secret count", test.name))
		for _, aSecret := range secrets {
			if aSecret.Name == secretA {
				assertSecretObject(t, aSecret, secretA, componentOneName, "Pending", test.name)
			}
			if aSecret.Name == secretB {
				assertSecretObject(t, aSecret, secretB, componentOneName, "Pending", test.name)
			}
			if aSecret.Name == secretC {
				assertSecretObject(t, aSecret, secretC, componentOneName, "Pending", test.name)
			}
			if aSecret.Name == secretD {
				assertSecretObject(t, aSecret, secretD, componentOneName, "Orphan", test.name)
			}
			if aSecret.Name == secretE {
				assertSecretObject(t, aSecret, secretE, componentOneName, "Orphan", test.name)
			}
			if aSecret.Name == secretF {
				assertSecretObject(t, aSecret, secretF, componentOneName, "Orphan", test.name)
			}
		}
	}
}

func TestGetEnvironmentSecrets_TwoComponents_AllConsistent(t *testing.T) {
	for _, test := range secretTestFunctions {
		commonTestUtils, _, kubeclient, radixclient, _ := setupTest()
		handler := initHandler(kubeclient, radixclient)

		appName := "any-app"
		componentOneName := "backend"
		componentTwoName := "frontend"
		environmentOne := "dev"
		buildFrom := "master"
		secretA := "a"
		secretB := "b"
		secretC := "c"

		componentOneSecrets := []string{secretA, secretB, secretC}
		componentTwoSecrets := []string{secretA, secretB, secretC}
		componentSecretsMap := map[string][]string{
			componentOneName: componentOneSecrets,
			componentTwoName: componentTwoSecrets,
		}

		clusterComponentOneSecrets := map[string][]byte{
			secretA: []byte(secretA),
			secretB: []byte(secretB),
			secretC: []byte(secretC),
		}
		clusterComponentTwoSecrets := map[string][]byte{
			secretA: []byte(secretA),
			secretB: []byte(secretB),
			secretC: []byte(secretC),
		}
		clusterComponentSecretsMap := map[string]map[string][]byte{
			componentOneName: clusterComponentOneSecrets,
			componentTwoName: clusterComponentTwoSecrets,
		}

		test.tester(commonTestUtils, kubeclient, appName, environmentOne, buildFrom, componentSecretsMap, clusterComponentSecretsMap)

		secrets, _ := handler.GetEnvironmentSecrets(appName, environmentOne)

		assert.Equal(t, 6, len(secrets), fmt.Sprintf("%s: incorrect secret count", test.name))
		for _, aSecret := range secrets {
			if aSecret.Component == componentOneName && aSecret.Name == secretA {
				assertSecretObject(t, aSecret, secretA, componentOneName, "Consistent", test.name)
			}
			if aSecret.Component == componentOneName && aSecret.Name == secretB {
				assertSecretObject(t, aSecret, secretB, componentOneName, "Consistent", test.name)
			}
			if aSecret.Component == componentOneName && aSecret.Name == secretC {
				assertSecretObject(t, aSecret, secretC, componentOneName, "Consistent", test.name)
			}
			if aSecret.Component == componentTwoName && aSecret.Name == secretA {
				assertSecretObject(t, aSecret, secretA, componentTwoName, "Consistent", test.name)
			}
			if aSecret.Component == componentTwoName && aSecret.Name == secretB {
				assertSecretObject(t, aSecret, secretB, componentTwoName, "Consistent", test.name)
			}
			if aSecret.Component == componentTwoName && aSecret.Name == secretC {
				assertSecretObject(t, aSecret, secretC, componentTwoName, "Consistent", test.name)
			}
		}
	}
}

func TestGetEnvironmentSecrets_TwoComponents_PartiallyConsistent(t *testing.T) {
	for _, test := range secretTestFunctions {
		commonTestUtils, _, kubeclient, radixclient, _ := setupTest()
		handler := initHandler(kubeclient, radixclient)

		appName := "any-app"
		componentOneName := "backend"
		componentTwoName := "frontend"
		environmentOne := "dev"
		buildFrom := "master"
		secretA := "a"
		secretB := "b"
		secretC := "c"
		secretD := "d"

		componentOneSecrets := []string{secretA, secretB, secretC}
		componentTwoSecrets := []string{secretA, secretB, secretC}
		componentSecretsMap := map[string][]string{
			componentOneName: componentOneSecrets,
			componentTwoName: componentTwoSecrets,
		}

		clusterComponentOneSecrets := map[string][]byte{
			secretB: []byte(secretB),
			secretC: []byte(secretC),
			secretD: []byte(secretD),
		}
		clusterComponentTwoSecrets := map[string][]byte{
			secretB: []byte(secretB),
			secretC: []byte(secretC),
			secretD: []byte(secretD),
		}
		clusterComponentSecretsMap := map[string]map[string][]byte{
			componentOneName: clusterComponentOneSecrets,
			componentTwoName: clusterComponentTwoSecrets,
		}

		test.tester(commonTestUtils, kubeclient, appName, environmentOne, buildFrom, componentSecretsMap, clusterComponentSecretsMap)

		secrets, _ := handler.GetEnvironmentSecrets(appName, environmentOne)

		assert.Equal(t, 8, len(secrets), fmt.Sprintf("%s: incorrect secret count", test.name))
		for _, aSecret := range secrets {
			if aSecret.Component == componentOneName && aSecret.Name == secretA {
				assertSecretObject(t, aSecret, secretA, componentOneName, "Pending", test.name)
			}
			if aSecret.Component == componentOneName && aSecret.Name == secretB {
				assertSecretObject(t, aSecret, secretB, componentOneName, "Consistent", test.name)
			}
			if aSecret.Component == componentOneName && aSecret.Name == secretC {
				assertSecretObject(t, aSecret, secretC, componentOneName, "Consistent", test.name)
			}
			if aSecret.Component == componentOneName && aSecret.Name == secretD {
				assertSecretObject(t, aSecret, secretD, componentOneName, "Orphan", test.name)
			}

			if aSecret.Component == componentTwoName && aSecret.Name == secretA {
				assertSecretObject(t, aSecret, secretA, componentTwoName, "Pending", test.name)
			}
			if aSecret.Component == componentTwoName && aSecret.Name == secretB {
				assertSecretObject(t, aSecret, secretB, componentTwoName, "Consistent", test.name)
			}
			if aSecret.Component == componentTwoName && aSecret.Name == secretC {
				assertSecretObject(t, aSecret, secretC, componentTwoName, "Consistent", test.name)
			}
			if aSecret.Component == componentTwoName && aSecret.Name == secretD {
				assertSecretObject(t, aSecret, secretD, componentTwoName, "Orphan", test.name)
			}
		}
	}
}

func TestGetEnvironmentSecrets_TwoComponents_NoConsistent(t *testing.T) {
	for _, test := range secretTestFunctions {
		commonTestUtils, _, kubeclient, radixclient, _ := setupTest()
		handler := initHandler(kubeclient, radixclient)

		appName := "any-app"
		componentOneName := "backend"
		componentTwoName := "frontend"
		environmentOne := "dev"
		buildFrom := "master"
		secretA := "a"
		secretB := "b"
		secretC := "c"
		secretD := "d"
		secretE := "e"
		secretF := "f"

		componentOneSecrets := []string{secretA, secretB, secretC}
		componentTwoSecrets := []string{secretA, secretB, secretC}
		componentSecretsMap := map[string][]string{
			componentOneName: componentOneSecrets,
			componentTwoName: componentTwoSecrets,
		}

		clusterComponentOneSecrets := map[string][]byte{
			secretD: []byte(secretD),
			secretE: []byte(secretE),
			secretF: []byte(secretF),
		}
		clusterComponentTwoSecrets := map[string][]byte{
			secretD: []byte(secretD),
			secretE: []byte(secretE),
			secretF: []byte(secretF),
		}
		clusterComponentSecretsMap := map[string]map[string][]byte{
			componentOneName: clusterComponentOneSecrets,
			componentTwoName: clusterComponentTwoSecrets,
		}

		test.tester(commonTestUtils, kubeclient, appName, environmentOne, buildFrom, componentSecretsMap, clusterComponentSecretsMap)

		secrets, _ := handler.GetEnvironmentSecrets(appName, environmentOne)

		assert.Equal(t, 12, len(secrets), fmt.Sprintf("%s: incorrect secret count", test.name))
		for _, aSecret := range secrets {
			if aSecret.Component == componentOneName && aSecret.Name == secretA {
				assertSecretObject(t, aSecret, secretA, componentOneName, "Pending", test.name)
			}
			if aSecret.Component == componentOneName && aSecret.Name == secretB {
				assertSecretObject(t, aSecret, secretB, componentOneName, "Pending", test.name)
			}
			if aSecret.Component == componentOneName && aSecret.Name == secretC {
				assertSecretObject(t, aSecret, secretC, componentOneName, "Pending", test.name)
			}
			if aSecret.Component == componentOneName && aSecret.Name == secretD {
				assertSecretObject(t, aSecret, secretD, componentOneName, "Orphan", test.name)
			}
			if aSecret.Component == componentOneName && aSecret.Name == secretE {
				assertSecretObject(t, aSecret, secretE, componentOneName, "Orphan", test.name)
			}
			if aSecret.Component == componentOneName && aSecret.Name == secretF {
				assertSecretObject(t, aSecret, secretF, componentOneName, "Orphan", test.name)
			}

			if aSecret.Component == componentTwoName && aSecret.Name == secretA {
				assertSecretObject(t, aSecret, secretA, componentTwoName, "Pending", test.name)
			}
			if aSecret.Component == componentTwoName && aSecret.Name == secretB {
				assertSecretObject(t, aSecret, secretB, componentTwoName, "Pending", test.name)
			}
			if aSecret.Component == componentTwoName && aSecret.Name == secretC {
				assertSecretObject(t, aSecret, secretC, componentTwoName, "Pending", test.name)
			}
			if aSecret.Component == componentTwoName && aSecret.Name == secretD {
				assertSecretObject(t, aSecret, secretD, componentTwoName, "Orphan", test.name)
			}
			if aSecret.Component == componentTwoName && aSecret.Name == secretE {
				assertSecretObject(t, aSecret, secretE, componentTwoName, "Orphan", test.name)
			}
			if aSecret.Component == componentTwoName && aSecret.Name == secretF {
				assertSecretObject(t, aSecret, secretF, componentTwoName, "Orphan", test.name)
			}
		}
	}
}

func contains(secrets []environmentModels.Secret, name string) bool {
	for _, secret := range secrets {
		if secret.Name == name {
			return true
		}
	}
	return false
}

func TestStopStartRestartComponent_ApplicationWithDeployment_EnvironmentConsistent(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, client, radixclient, _ := setupTest()

	anyAppName := "any-app"
	anyEnvironment := "dev"

	rd, _ := commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithRadixApplication(builders.
			ARadixApplication().
			WithRadixRegistration(builders.ARadixRegistration()).
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment, "master")).
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment))

	componentName := rd.Spec.Components[0].Name

	// Test
	zeroReplicas := 0
	assert.True(t, *rd.Spec.Components[0].Replicas > zeroReplicas)

	responseChannel := controllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/stop", anyAppName, anyEnvironment, componentName))
	response := <-responseChannel

	// Since pods are not appearing out of nowhere with kubernetes-fake, the component will be in
	// a reconciling state because number of replicas in spec > 0. Therefore it can be stopped
	assert.Equal(t, http.StatusOK, response.Code)

	updatedRd, _ := radixclient.RadixV1().RadixDeployments(rd.GetNamespace()).Get(context.TODO(), rd.GetName(), metav1.GetOptions{})
	assert.True(t, *updatedRd.Spec.Components[0].Replicas == zeroReplicas)

	responseChannel = controllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/stop", anyAppName, anyEnvironment, componentName))
	response = <-responseChannel

	// The component is in a stopped state since replicas in spec = 0, and therefore cannot be stopped again
	assert.Equal(t, http.StatusBadRequest, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	expectedError := environmentModels.CannotStopComponent(anyAppName, anyComponentName, deploymentModels.StoppedComponent.String())
	assert.Equal(t, (expectedError.(*radixhttp.Error)).Message, errorResponse.Message)

	// Create pod
	createComponentPod(client, rd.GetNamespace(), componentName)

	responseChannel = controllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/start", anyAppName, anyEnvironment, componentName))
	response = <-responseChannel

	// Since pods are not appearing out of nowhere with kubernetes-fake, the component will be in
	// a reconciling state and cannot be started
	assert.Equal(t, http.StatusBadRequest, response.Code)
	errorResponse, _ = controllertest.GetErrorResponse(response)
	expectedError = environmentModels.CannotStartComponent(anyAppName, anyComponentName, deploymentModels.ComponentReconciling.String())
	assert.Equal(t, (expectedError.(*radixhttp.Error)).Message, errorResponse.Message)

	// Emulate a stopped component
	deleteComponentPod(client, rd.GetNamespace(), componentName)

	responseChannel = controllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/start", anyAppName, anyEnvironment, componentName))
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)

	updatedRd, _ = radixclient.RadixV1().RadixDeployments(rd.GetNamespace()).Get(context.TODO(), rd.GetName(), metav1.GetOptions{})
	assert.True(t, *updatedRd.Spec.Components[0].Replicas != zeroReplicas)

	responseChannel = controllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/restart", anyAppName, anyEnvironment, componentName))
	response = <-responseChannel

	// Since pods are not appearing out of nowhere with kubernetes-fake, the component will be in
	// a reconciling state and cannot be restarted
	assert.Equal(t, http.StatusBadRequest, response.Code)
	errorResponse, _ = controllertest.GetErrorResponse(response)
	expectedError = environmentModels.CannotRestartComponent(anyAppName, anyComponentName, deploymentModels.ComponentReconciling.String())
	assert.Equal(t, (expectedError.(*radixhttp.Error)).Message, errorResponse.Message)

	// Emulate a started component
	createComponentPod(client, rd.GetNamespace(), componentName)

	responseChannel = controllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/restart", anyAppName, anyEnvironment, componentName))
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)

	updatedRd, _ = radixclient.RadixV1().RadixDeployments(rd.GetNamespace()).Get(context.TODO(), rd.GetName(), metav1.GetOptions{})
	assert.True(t, *updatedRd.Spec.Components[0].Replicas != zeroReplicas)
	assert.NotEmpty(t, updatedRd.Spec.Components[0].EnvironmentVariables[defaults.RadixRestartEnvironmentVariable])
}

func TestCreateEnvironment(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _, _ := setupTest()

	appName := "myApp"
	envName := "myEnv"

	commonTestUtils.ApplyApplication(builders.
		ARadixApplication().
		WithAppName(appName))

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("POST", fmt.Sprintf("/api/v1/applications/%s/environments/%s", appName, envName))
	response := <-responseChannel

	assert.Equal(t, http.StatusOK, response.Code)
}

func TestGetEnvironmentSecretsForDeploymentForExternalAlias(t *testing.T) {
	commonTestUtils, _, kubeclient, radixclient, _ := setupTest()
	handler := initHandler(kubeclient, radixclient)

	appName := "any-app"
	componentName := "backend"
	environmentName := "dev"
	buildFrom := "master"
	alias := "cdn.myalias.com"

	deployment, err := commonTestUtils.ApplyDeployment(builders.
		ARadixDeployment().
		WithAppName(appName).
		WithEnvironment(environmentName).
		WithImageTag(buildFrom))

	commonTestUtils.ApplyApplication(builders.
		ARadixApplication().
		WithAppName(appName).
		WithEnvironment(environmentName, buildFrom).
		WithComponent(builders.
			AnApplicationComponent().
			WithName(componentName)).
		WithDNSExternalAlias(alias, environmentName, componentName))

	secrets, err := handler.GetEnvironmentSecretsForDeployment(appName, environmentName, &deploymentModels.Deployment{
		Name: deployment.Name,
	})

	assert.NoError(t, err)
	assert.Len(t, secrets, 2)
	for _, s := range secrets {
		if s.Name == alias+"-key" {
			assert.Equal(t, "Pending", s.Status)
		} else if s.Name == alias+"-cert" {
			assert.Equal(t, "Pending", s.Status)
		}
	}

	kubeclient.CoreV1().Secrets(appName+"-"+environmentName).Create(context.TODO(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: alias,
		},
	}, metav1.CreateOptions{})

	secrets, err = handler.GetEnvironmentSecretsForDeployment(appName, environmentName, &deploymentModels.Deployment{
		Name: deployment.Name,
	})

	assert.NoError(t, err)
	assert.Len(t, secrets, 2)
	for _, s := range secrets {
		if s.Name == alias+"-key" {
			assert.Equal(t, "Consistent", s.Status)
		} else if s.Name == alias+"-cert" {
			assert.Equal(t, "Consistent", s.Status)
		}
	}
}

func Test_GetEnvironmentEvents_Controller(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeClient, _, _ := setupTest()
	anyAppName := "any-app"
	createEvent := func(namespace, eventName string) {
		kubeClient.CoreV1().Events(namespace).CreateWithEventNamespace(&corev1.Event{
			ObjectMeta: metav1.ObjectMeta{
				Name: eventName,
			},
		})
	}
	createEvent(k8sObjectUtils.GetEnvironmentNamespace(anyAppName, "dev"), "ev1")
	createEvent(k8sObjectUtils.GetEnvironmentNamespace(anyAppName, "dev"), "ev2")
	commonTestUtils.ApplyApplication(builders.
		ARadixApplication().
		WithAppName(anyAppName).
		WithEnvironment("dev", "master"))

	t.Run("Get events for dev environment", func(t *testing.T) {
		responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/events", anyAppName, "dev"))
		response := <-responseChannel
		assert.Equal(t, http.StatusOK, response.Code)
		events := make([]eventModels.Event, 0)
		controllertest.GetResponseBody(response, &events)
		assert.Len(t, events, 2)
	})

	t.Run("Get events for non-existing environment", func(t *testing.T) {
		responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/events", anyAppName, "prod"))
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
		responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s/events", "noapp", "dev"))
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

func Test_GetEnvironmentEvents_Handler(t *testing.T) {
	appName, envName := "app", "dev"
	commonTestUtils, _, kubeclient, radixclient, _ := setupTest()
	ctrl := gomock.NewController(t)
	ctrl.Finish()
	eventHandler := eventMock.NewMockEventHandler(ctrl)
	handler := initHandler(kubeclient, radixclient, WithEventHandler(eventHandler))
	raBuilder := builders.ARadixApplication().WithAppName(appName).WithEnvironment(envName, "master")
	commonTestUtils.ApplyApplication(raBuilder)
	nsFunc := event.RadixEnvironmentNamespace(raBuilder.BuildRA(), envName)
	eventHandler.EXPECT().
		GetEvents(test.EqualsNamespaceFunc(nsFunc)).
		Return(make([]*eventModels.Event, 0), fmt.Errorf("err")).
		Return([]*eventModels.Event{{}, {}}, nil).
		Times(1)

	events, err := handler.GetEnvironmentEvents(appName, envName)
	assert.Nil(t, err)
	assert.Len(t, events, 2)
}

func initHandler(client kubernetes.Interface,
	radixclient radixclient.Interface,
	handlerConfig ...EnvironmentHandlerOptions) EnvironmentHandler {
	accounts := models.NewAccounts(client, radixclient, client, radixclient, "", radixmodels.Impersonation{})
	options := []EnvironmentHandlerOptions{WithAccounts(accounts)}
	options = append(options, handlerConfig...)
	return Init(options...)
}

func createComponentPod(kubeclient kubernetes.Interface, namespace, componentName string) {
	podSpec := getPodSpec(componentName)
	kubeclient.CoreV1().Pods(namespace).Create(context.TODO(), podSpec, metav1.CreateOptions{})
}

func deleteComponentPod(kubeclient kubernetes.Interface, namespace, componentName string) {
	kubeclient.CoreV1().Pods(namespace).Delete(context.TODO(), getComponentPodName(componentName), metav1.DeleteOptions{})
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
