package applications

import (
	"context"
	"fmt"
	"github.com/equinor/radix-api/api/utils"
	"net/http"
	"net/url"
	"os"
	strings "strings"
	"testing"
	"time"

	applicationModels "github.com/equinor/radix-api/api/applications/models"
	environmentModels "github.com/equinor/radix-api/api/environments/models"
	jobModels "github.com/equinor/radix-api/api/jobs/models"
	controllertest "github.com/equinor/radix-api/api/test"
	radixhttp "github.com/equinor/radix-common/net/http"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	jobPipeline "github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	builders "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	prometheusclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

const (
	clusterName       = "AnyClusterName"
	containerRegistry = "any.container.registry"
	dnsZone           = "dev.radix.equinor.com"
	appAliasDNSZone   = "app.dev.radix.equinor.com"
)

func setupTest() (*commontest.Utils, *controllertest.Utils, *kubefake.Clientset, *fake.Clientset, prometheusclient.Interface) {
	// Setup
	kubeclient := kubefake.NewSimpleClientset()
	radixclient := fake.NewSimpleClientset()
	prometheusclient := prometheusfake.NewSimpleClientset()

	// commonTestUtils is used for creating CRDs
	commonTestUtils := commontest.NewTestUtils(kubeclient, radixclient)
	commonTestUtils.CreateClusterPrerequisites(clusterName, containerRegistry)
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, clusterName)

	// controllerTestUtils is used for issuing HTTP request and processing responses
	controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, NewApplicationController(func(client kubernetes.Interface, rr v1.RadixRegistration) bool { return true }))

	return &commonTestUtils, &controllerTestUtils, kubeclient, radixclient, prometheusclient
}

func TestGetApplications_HasAccessToSomeRR(t *testing.T) {
	commonTestUtils, _, kubeclient, radixclient, _ := setupTest()

	commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithCloneURL("git@github.com:Equinor/my-app.git"))
	commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithCloneURL("git@github.com:Equinor/my-second-app.git").WithAdGroups([]string{"2"}).WithName("my-second-app"))

	t.Run("no access", func(t *testing.T) {
		controllerTestUtils := controllertest.NewTestUtils(
			kubeclient,
			radixclient,
			NewApplicationController(
				func(client kubernetes.Interface, rr v1.RadixRegistration) bool {
					return false
				}))
		responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/applications")
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		controllertest.GetResponseBody(response, &applications)
		assert.Equal(t, 0, len(applications))
	})

	t.Run("access to single app", func(t *testing.T) {
		controllerTestUtils := controllertest.NewTestUtils(
			kubeclient,
			radixclient,
			NewApplicationController(
				func(client kubernetes.Interface, rr v1.RadixRegistration) bool {
					return rr.GetName() == "my-second-app"
				}))
		responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/applications")
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		controllertest.GetResponseBody(response, &applications)
		assert.Equal(t, 1, len(applications))
	})

	t.Run("access to all app", func(t *testing.T) {
		controllerTestUtils := controllertest.NewTestUtils(
			kubeclient,
			radixclient,
			NewApplicationController(
				func(client kubernetes.Interface, rr v1.RadixRegistration) bool {
					return true
				}))
		responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/applications")
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		controllertest.GetResponseBody(response, &applications)
		assert.Equal(t, 2, len(applications))
	})
}

func TestGetApplications_WithFilterOnSSHRepo_Filter(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _, _ := setupTest()
	commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithCloneURL("git@github.com:Equinor/my-app.git"))

	// Test
	t.Run("matching repo", func(t *testing.T) {
		responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications?sshRepo=%s", url.QueryEscape("git@github.com:Equinor/my-app.git")))
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		controllertest.GetResponseBody(response, &applications)
		assert.Equal(t, 1, len(applications))
	})

	t.Run("unmatching repo", func(t *testing.T) {
		responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications?sshRepo=%s", url.QueryEscape("git@github.com:Equinor/my-app2.git")))
		response := <-responseChannel

		applications := make([]*applicationModels.ApplicationSummary, 0)
		controllertest.GetResponseBody(response, &applications)
		assert.Equal(t, 0, len(applications))
	})

	t.Run("no filter", func(t *testing.T) {
		responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/applications")
		response := <-responseChannel

		applications := make([]*applicationModels.ApplicationSummary, 0)
		controllertest.GetResponseBody(response, &applications)
		assert.Equal(t, 1, len(applications))
	})
}

func TestCreateApplication_NoName_ValidationError(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _ := setupTest()

	// Test
	parameters := AnApplicationRegistration().withName("").Build()
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusBadRequest, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	assert.Equal(t, "Error: app name cannot be empty", errorResponse.Message)
}

func TestCreateApplication_WhenRepoIsNotSet_DoNotGenerateDeployKey(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _ := setupTest()

	// Test
	parameters := AnApplicationRegistration().withRepository("").Build()
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	response := <-responseChannel

	application := applicationModels.ApplicationRegistration{}
	controllertest.GetResponseBody(response, &application)
	assert.Equal(t, "", application.PublicKey)
}

func TestCreateApplication_WhenRepoIsSetAnDeployKeyIsNot_GenerateDeployKey(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _ := setupTest()

	// Test
	parameters := AnApplicationRegistration().
		withName("any-name-1").
		withRepository("https://github.com/Equinor/any-repo").
		Build()
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	response := <-responseChannel

	application := applicationModels.ApplicationRegistration{}
	controllertest.GetResponseBody(response, &application)
	assert.NotEmpty(t, application.PublicKey)
}

func TestCreateApplication_WhenOnlyOnePartOfDeployKeyIsSet_ReturnError(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _ := setupTest()

	// Test
	parameters := AnApplicationRegistration().
		withName("any-name-2").
		withRepository("https://github.com/Equinor/any-repo").
		withPublicKey("Any public key").
		Build()
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusBadRequest, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	expectedError := applicationModels.OnePartOfDeployKeyIsNotAllowed()
	assert.Equal(t, (expectedError.(*radixhttp.Error)).Message, errorResponse.Message)

	parameters = AnApplicationRegistration().
		withName("any-name-2").
		withRepository("https://github.com/Equinor/any-repo").
		withPrivateKey("Any private key").
		Build()
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	response = <-responseChannel

	assert.Equal(t, http.StatusBadRequest, response.Code)
	errorResponse, _ = controllertest.GetErrorResponse(response)
	expectedError = applicationModels.OnePartOfDeployKeyIsNotAllowed()
	assert.Equal(t, (expectedError.(*radixhttp.Error)).Message, errorResponse.Message)
}

func TestCreateApplication_WhenDeployKeyIsSet_DoNotGenerateDeployKey(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _ := setupTest()

	// Test
	parameters := AnApplicationRegistration().
		withName("any-name-2").
		withRepository("https://github.com/Equinor/any-repo").
		withPublicKey("Any public key").
		withPrivateKey("Any private key").
		Build()
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	response := <-responseChannel

	application := applicationModels.ApplicationRegistration{}
	controllertest.GetResponseBody(response, &application)
	assert.Equal(t, "Any public key", application.PublicKey)
}

func TestCreateApplication_WhenOwnerIsNotSet_ReturnError(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _ := setupTest()

	// Test
	parameters := AnApplicationRegistration().
		withName("any-name-2").
		withRepository("https://github.com/Equinor/any-repo").
		withPublicKey("Any public key").
		withPrivateKey("Any private key").
		withOwner("").
		Build()
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusBadRequest, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	expectedError := radixvalidators.InvalidEmailError("owner", "")
	assert.Equal(t, fmt.Sprintf("Error: %v", expectedError), errorResponse.Message)
}

func TestCreateApplication_WhenConfigBranchIsNotSet_ReturnError(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _ := setupTest()

	// Test
	parameters := AnApplicationRegistration().
		withName("any-name").
		withRepository("https://github.com/Equinor/any-repo").
		withPublicKey("Any public key").
		withPrivateKey("Any private key").
		withConfigBranch("").
		Build()
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusBadRequest, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	expectedError := radixvalidators.ResourceNameCannotBeEmptyError("branch name")
	assert.Equal(t, fmt.Sprintf("Error: %v", expectedError), errorResponse.Message)
}

func TestCreateApplication_WhenConfigBranchIsInvalid_ReturnError(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _ := setupTest()

	// Test
	configBranch := "main.."
	parameters := AnApplicationRegistration().
		withName("any-name").
		withRepository("https://github.com/Equinor/any-repo").
		withPublicKey("Any public key").
		withPrivateKey("Any private key").
		withConfigBranch(configBranch).
		Build()
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusBadRequest, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	expectedError := radixvalidators.InvalidConfigBranchName(configBranch)
	assert.Equal(t, fmt.Sprintf("Error: %v", expectedError), errorResponse.Message)
}

func TestGetApplication_ShouldNeverReturnPrivatePartOfDeployKey(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _, _ := setupTest()
	commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithName("some-app").
		WithPublicKey("some-public-key").
		WithPrivateKey("some-private-key"))

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "some-app"))
	response := <-responseChannel

	application := applicationModels.Application{}
	controllertest.GetResponseBody(response, &application)

	assert.Equal(t, "", application.Registration.PrivateKey)
}

func TestCreateApplication_DuplicateRepo_ShouldFailAsWeCannotHandleThatSituation(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _ := setupTest()

	parameters := AnApplicationRegistration().
		withName("any-name").
		withRepository("https://github.com/Equinor/any-repo").
		Build()
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	<-responseChannel

	// Test
	parameters = AnApplicationRegistration().
		withName("any-other-name").
		withRepository("https://github.com/Equinor/any-repo").
		Build()
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusBadRequest, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	assert.Equal(t, "Error: Repository is in use by any-name", errorResponse.Message)
}

func TestGetApplication_AllFieldsAreSet(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _ := setupTest()

	parameters := AnApplicationRegistration().
		withName("any-name").
		withRepository("https://github.com/Equinor/any-repo").
		withSharedSecret("Any secret").
		withAdGroups([]string{"a6a3b81b-34gd-sfsf-saf2-7986371ea35f"}).
		withOwner("AN_OWNER@equinor.com").
		withWBS("A.BCD.00.999").
		withConfigBranch("abranch").
		Build()

	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	<-responseChannel

	// Test
	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response := <-responseChannel

	application := applicationModels.Application{}
	controllertest.GetResponseBody(response, &application)

	assert.Equal(t, "https://github.com/Equinor/any-repo", application.Registration.Repository)
	assert.Equal(t, "Any secret", application.Registration.SharedSecret)
	assert.Equal(t, []string{"a6a3b81b-34gd-sfsf-saf2-7986371ea35f"}, application.Registration.AdGroups)
	assert.Equal(t, "AN_OWNER@equinor.com", application.Registration.Owner)
	assert.Equal(t, "RADIX@equinor.com", application.Registration.Creator)
	assert.Equal(t, "A.BCD.00.999", application.Registration.WBS)
	assert.Equal(t, "abranch", application.Registration.ConfigBranch)
}

func TestGetApplications_WithJobs_ShouldOnlyHaveLatest(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _, _ := setupTest()

	commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithName("app-1"))
	commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithName("app-2"))
	commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithName("app-3"))

	commontest.CreateAppNamespace(kubeclient, "app-1")
	commontest.CreateAppNamespace(kubeclient, "app-2")
	commontest.CreateAppNamespace(kubeclient, "app-3")

	app1Job1Started, _ := radixutils.ParseTimestamp("2018-11-12T11:45:26Z")
	app2Job1Started, _ := radixutils.ParseTimestamp("2018-11-12T12:30:14Z")
	app2Job2Started, _ := radixutils.ParseTimestamp("2018-11-20T09:00:00Z")
	app2Job3Started, _ := radixutils.ParseTimestamp("2018-11-20T09:00:01Z")

	createRadixJob(commonTestUtils, "app-1", "app-1-job-1", app1Job1Started)
	createRadixJob(commonTestUtils, "app-2", "app-2-job-1", app2Job1Started)
	createRadixJob(commonTestUtils, "app-2", "app-2-job-2", app2Job2Started)
	createRadixJob(commonTestUtils, "app-2", "app-2-job-3", app2Job3Started)

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/applications")
	response := <-responseChannel

	applications := make([]*applicationModels.ApplicationSummary, 0)
	controllertest.GetResponseBody(response, &applications)

	for _, application := range applications {
		if strings.EqualFold(application.Name, "app-1") {
			assert.NotNil(t, application.LatestJob)
			assert.Equal(t, "app-1-job-1", application.LatestJob.Name)
		} else if strings.EqualFold(application.Name, "app-2") {
			assert.NotNil(t, application.LatestJob)
			assert.Equal(t, "app-2-job-3", application.LatestJob.Name)
		} else if strings.EqualFold(application.Name, "app-3") {
			assert.Nil(t, application.LatestJob)
		}
	}
}

func TestGetApplication_WithJobs(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _, _ := setupTest()
	commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithName("any-name"))

	commontest.CreateAppNamespace(kubeclient, "any-name")
	app1Job1Started, _ := radixutils.ParseTimestamp("2018-11-12T11:45:26Z")
	app1Job2Started, _ := radixutils.ParseTimestamp("2018-11-12T12:30:14Z")
	app1Job3Started, _ := radixutils.ParseTimestamp("2018-11-20T09:00:00Z")

	createRadixJob(commonTestUtils, "any-name", "any-name-job-1", app1Job1Started)
	createRadixJob(commonTestUtils, "any-name", "any-name-job-2", app1Job2Started)
	createRadixJob(commonTestUtils, "any-name", "any-name-job-3", app1Job3Started)

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response := <-responseChannel

	application := applicationModels.Application{}
	controllertest.GetResponseBody(response, &application)
	assert.Equal(t, 3, len(application.Jobs))
}

func TestGetApplication_WithEnvironments(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, radix, _ := setupTest()

	anyAppName := "any-app"
	anyOrphanedEnvironment := "feature"

	commonTestUtils.ApplyRegistration(builders.
		NewRegistrationBuilder().
		WithName(anyAppName))

	commonTestUtils.ApplyApplication(builders.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment("dev", "master").
		WithEnvironment("prod", "release"))

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

	re, _ := commonTestUtils.ApplyEnvironment(builders.
		NewEnvironmentBuilder().
		WithAppLabel().
		WithAppName(anyAppName).
		WithEnvironmentName(anyOrphanedEnvironment))

	re.Status.Orphaned = true
	radix.RadixV1().RadixEnvironments().Update(context.TODO(), re, metav1.UpdateOptions{})

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", anyAppName))
	response := <-responseChannel

	application := applicationModels.Application{}
	controllertest.GetResponseBody(response, &application)
	assert.Equal(t, 3, len(application.Environments))

	for _, environment := range application.Environments {
		if strings.EqualFold(environment.Name, "dev") {
			assert.Equal(t, environmentModels.Consistent.String(), environment.Status)
			assert.NotNil(t, environment.ActiveDeployment)
		} else if strings.EqualFold(environment.Name, "prod") {
			assert.Equal(t, environmentModels.Pending.String(), environment.Status)
			assert.Nil(t, environment.ActiveDeployment)
		} else if strings.EqualFold(environment.Name, anyOrphanedEnvironment) {
			assert.Equal(t, environmentModels.Orphan.String(), environment.Status)
			assert.NotNil(t, environment.ActiveDeployment)
		}
	}

}

func TestUpdateApplication_DuplicateRepo_ShouldFailAsWeCannotHandleThatSituation(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _ := setupTest()

	parameters := AnApplicationRegistration().
		withName("any-name").
		withRepository("https://github.com/Equinor/any-repo").
		Build()

	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	<-responseChannel

	parameters = AnApplicationRegistration().
		withName("any-other-name").
		withRepository("https://github.com/Equinor/any-other-repo").
		Build()

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	<-responseChannel

	// Test
	parameters = AnApplicationRegistration().
		withName("any-other-name").
		withRepository("https://github.com/Equinor/any-repo").
		Build()

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "any-other-name"), parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusBadRequest, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	assert.Equal(t, "Error: Repository is in use by any-name", errorResponse.Message)
}

func TestUpdateApplication_MismatchingNameOrNotExists_ShouldFailAsIllegalOperation(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _ := setupTest()

	parameters := AnApplicationRegistration().withName("any-name").Build()
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	<-responseChannel

	// Test
	parameters = AnApplicationRegistration().withName("any-name").Build()
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "another-name"), parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusNotFound, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	assert.Equal(t, controllertest.AppNotFoundErrorMsg("another-name"), errorResponse.Message)

	parameters = AnApplicationRegistration().withName("another-name").Build()
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "any-name"), parameters)
	response = <-responseChannel

	assert.Equal(t, http.StatusBadRequest, response.Code)
	errorResponse, _ = controllertest.GetErrorResponse(response)
	assert.Equal(t, "App name any-name does not correspond with application name another-name", errorResponse.Message)

	parameters = AnApplicationRegistration().withName("another-name").Build()
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "another-name"), parameters)
	response = <-responseChannel
	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestUpdateApplication_AbleToSetAnySpecField(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _ := setupTest()

	builder := AnApplicationRegistration().
		withName("any-name").
		withRepository("https://github.com/Equinor/a-repo").
		withSharedSecret("").
		withPublicKey("")
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", builder.Build())
	<-responseChannel

	// Test Repository
	newRepository := "https://github.com/Equinor/any-repo"
	builder = builder.
		withRepository(newRepository)

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "any-name"), builder.Build())
	response := <-responseChannel

	application := applicationModels.ApplicationRegistration{}
	controllertest.GetResponseBody(response, &application)
	assert.Equal(t, newRepository, application.Repository)

	// Test SharedSecret
	newSharedSecret := "Any shared secret"
	builder = builder.
		withSharedSecret(newSharedSecret)

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "any-name"), builder.Build())
	response = <-responseChannel
	controllertest.GetResponseBody(response, &application)
	assert.Equal(t, newSharedSecret, application.SharedSecret)

	// Test PublicKey
	newPublicKey := "Any public key"
	builder = builder.
		withPublicKey(newPublicKey)

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "any-name"), builder.Build())
	response = <-responseChannel
	controllertest.GetResponseBody(response, &application)
	assert.Equal(t, newPublicKey, application.PublicKey)

	// Test WBS
	newWbs := "new.wbs.code"
	builder = builder.
		withWBS(newWbs)

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "any-name"), builder.Build())
	response = <-responseChannel
	controllertest.GetResponseBody(response, &application)
	assert.Equal(t, newWbs, application.WBS)

	// Test ConfigBranch
	newConfigBranch := "newcfgbranch"
	builder = builder.
		withConfigBranch(newConfigBranch)

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "any-name"), builder.Build())
	response = <-responseChannel
	controllertest.GetResponseBody(response, &application)
	assert.Equal(t, newConfigBranch, application.ConfigBranch)
}

func TestModifyApplication_AbleToSetField(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _ := setupTest()

	builder := AnApplicationRegistration().
		withName("any-name").
		withRepository("https://github.com/Equinor/a-repo").
		withSharedSecret("").
		withPublicKey("").
		withAdGroups([]string{"a5dfa635-dc00-4a28-9ad9-9e7f1e56919d"}).
		withOwner("AN_OWNER@equinor.com").
		withWBS("T.O123A.AZ.45678").
		withConfigBranch("main1")
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", builder.Build())
	<-responseChannel

	// Test
	anyNewAdGroup := []string{"98765432-dc00-4a28-9ad9-9e7f1e56919d"}
	patchRequest := applicationModels.ApplicationPatchRequest{
		AdGroups: &anyNewAdGroup,
	}

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PATCH", fmt.Sprintf("/api/v1/applications/%s", "any-name"), patchRequest)
	<-responseChannel

	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response := <-responseChannel

	application := applicationModels.Application{}
	controllertest.GetResponseBody(response, &application)
	assert.Equal(t, anyNewAdGroup, application.Registration.AdGroups)
	assert.Equal(t, "AN_OWNER@equinor.com", application.Registration.Owner)
	assert.Equal(t, "T.O123A.AZ.45678", application.Registration.WBS)
	assert.Equal(t, "main1", application.Registration.ConfigBranch)

	// Test
	anyNewOwner := "A_NEW_OWNER@equinor.com"
	patchRequest = applicationModels.ApplicationPatchRequest{
		Owner: &anyNewOwner,
	}

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PATCH", fmt.Sprintf("/api/v1/applications/%s", "any-name"), patchRequest)
	<-responseChannel

	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response = <-responseChannel

	controllertest.GetResponseBody(response, &application)
	assert.Equal(t, anyNewAdGroup, application.Registration.AdGroups)
	assert.Equal(t, anyNewOwner, application.Registration.Owner)

	// Test
	anyNewAdGroup = []string{}
	patchRequest = applicationModels.ApplicationPatchRequest{
		AdGroups: &anyNewAdGroup,
	}

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PATCH", fmt.Sprintf("/api/v1/applications/%s", "any-name"), patchRequest)
	<-responseChannel

	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response = <-responseChannel

	controllertest.GetResponseBody(response, &application)
	assert.Nil(t, application.Registration.AdGroups)
	assert.Equal(t, anyNewOwner, application.Registration.Owner)

	// Test
	anyNewWBS := "A.BCD.00.999"
	patchRequest = applicationModels.ApplicationPatchRequest{
		WBS: &anyNewWBS,
	}

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PATCH", fmt.Sprintf("/api/v1/applications/%s", "any-name"), patchRequest)
	<-responseChannel

	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response = <-responseChannel

	controllertest.GetResponseBody(response, &application)
	assert.Equal(t, anyNewWBS, application.Registration.WBS)

	// Test ConfigBranch
	anyNewConfigBranch := "main2"
	patchRequest = applicationModels.ApplicationPatchRequest{
		ConfigBranch: &anyNewConfigBranch,
	}

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PATCH", fmt.Sprintf("/api/v1/applications/%s", "any-name"), patchRequest)
	<-responseChannel

	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response = <-responseChannel

	controllertest.GetResponseBody(response, &application)
	assert.Equal(t, anyNewConfigBranch, application.Registration.ConfigBranch)
}

func TestModifyApplication_AbleToUpdateRepository(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _ := setupTest()

	builder := AnApplicationRegistration().
		withName("any-name").
		withRepository("https://github.com/Equinor/a-repo")
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", builder.Build())
	<-responseChannel

	// Test
	anyNewRepo := "https://github.com/repo/updated-version"
	patchRequest := applicationModels.ApplicationPatchRequest{
		Repository: &anyNewRepo,
	}

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PATCH", fmt.Sprintf("/api/v1/applications/%s", "any-name"), patchRequest)
	<-responseChannel

	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response := <-responseChannel

	application := applicationModels.Application{}
	controllertest.GetResponseBody(response, &application)
	assert.Equal(t, anyNewRepo, application.Registration.Repository)
}

func TestModifyApplication_ConfigBranchSetToFallbackHack(t *testing.T) {
	// Setup
	appName := "any-name"
	_, controllerTestUtils, _, radixClient, _ := setupTest()
	rr := builders.ARadixRegistration().
		WithName(appName).
		WithConfigBranch("")
	radixClient.RadixV1().RadixRegistrations().Create(context.TODO(), rr.BuildRR(), metav1.CreateOptions{})

	// Test
	anyNewRepo := "https://github.com/repo/updated-version"
	patchRequest := applicationModels.ApplicationPatchRequest{
		Repository: &anyNewRepo,
	}

	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("PATCH", fmt.Sprintf("/api/v1/applications/%s", appName), patchRequest)
	<-responseChannel

	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", appName))
	response := <-responseChannel

	application := applicationModels.Application{}
	controllertest.GetResponseBody(response, &application)
	assert.Equal(t, applicationconfig.ConfigBranchFallback, application.Registration.ConfigBranch)
}

func TestHandleTriggerPipeline_ForNonMappedAndMappedAndMagicBranchEnvironment_JobIsNotCreatedForUnmapped(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _, _ := setupTest()
	anyAppName := "any-app"
	configBranch := "magic"

	rr := builders.ARadixRegistration().WithConfigBranch(configBranch)
	commonTestUtils.ApplyApplication(builders.
		ARadixApplication().
		WithRadixRegistration(rr).
		WithAppName(anyAppName).
		WithEnvironment("dev", "dev").
		WithEnvironment("prod", "release"),
	)

	// Test
	unmappedBranch := "feature"

	parameters := applicationModels.PipelineParametersBuild{Branch: unmappedBranch}
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/pipelines/%s", anyAppName, v1.BuildDeploy), parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusBadRequest, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	expectedError := applicationModels.UnmatchedBranchToEnvironment(unmappedBranch)
	assert.Equal(t, (expectedError.(*radixhttp.Error)).Message, errorResponse.Message)

	// Mapped branch should start job
	parameters = applicationModels.PipelineParametersBuild{Branch: "dev"}
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/pipelines/%s", anyAppName, v1.BuildDeploy), parameters)
	response = <-responseChannel

	assert.Equal(t, http.StatusOK, response.Code)

	// Magic branch should start job, even if it is not mapped
	parameters = applicationModels.PipelineParametersBuild{Branch: configBranch}
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/pipelines/%s", anyAppName, v1.BuildDeploy), parameters)
	response = <-responseChannel

	assert.Equal(t, http.StatusOK, response.Code)
}

func TestHandleTriggerPipeline_ExistingAndNonExistingApplication_JobIsCreatedForExisting(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _ := setupTest()

	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", AnApplicationRegistration().
		withName("any-app").withConfigBranch("maincfg").Build())
	<-responseChannel

	// Test
	const pushCommitID = "4faca8595c5283a9d0f17a623b9255a0d9866a2e"

	parameters := applicationModels.PipelineParametersBuild{Branch: "master", CommitID: pushCommitID}
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/pipelines/%s", "another-app", v1.BuildDeploy), parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusNotFound, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	assert.Equal(t, controllertest.AppNotFoundErrorMsg("another-app"), errorResponse.Message)

	parameters = applicationModels.PipelineParametersBuild{Branch: "", CommitID: pushCommitID}
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/pipelines/%s", "any-app", v1.BuildDeploy), parameters)
	response = <-responseChannel

	assert.Equal(t, http.StatusBadRequest, response.Code)
	errorResponse, _ = controllertest.GetErrorResponse(response)
	expectedError := applicationModels.AppNameAndBranchAreRequiredForStartingPipeline()
	assert.Equal(t, (expectedError.(*radixhttp.Error)).Message, errorResponse.Message)

	parameters = applicationModels.PipelineParametersBuild{Branch: "maincfg", CommitID: pushCommitID}
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/pipelines/%s", "any-app", v1.BuildDeploy), parameters)
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)

	jobSummary := jobModels.JobSummary{}
	controllertest.GetResponseBody(response, &jobSummary)
	assert.Equal(t, "any-app", jobSummary.AppName)
	assert.Equal(t, "maincfg", jobSummary.Branch)
	assert.Equal(t, pushCommitID, jobSummary.CommitID)
}

func TestHandleTriggerPipeline_Deploy_JobHasCorrectParameters(t *testing.T) {
	_, controllerTestUtils, _, radixclient, _ := setupTest()

	appName := "an-app"

	parameters := applicationModels.PipelineParametersDeploy{
		ToEnvironment: "target",
	}

	<-controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", AnApplicationRegistration().withName(appName).Build())
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/pipelines/%s", appName, v1.Deploy), parameters)
	<-responseChannel

	appNamespace := fmt.Sprintf("%s-app", appName)
	jobs, _ := getJobsInNamespace(radixclient, appNamespace)

	assert.Equal(t, jobs[0].Spec.Deploy.ToEnvironment, "target")
}

func TestHandleTriggerPipeline_Promote_JobHasCorrectParameters(t *testing.T) {
	_, controllerTestUtils, _, radixclient, _ := setupTest()

	appName := "an-app"

	parameters := applicationModels.PipelineParametersPromote{
		FromEnvironment: "origin",
		ToEnvironment:   "target",
		DeploymentName:  "a-deployment",
	}

	<-controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", AnApplicationRegistration().withName(appName).Build())
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/pipelines/%s", appName, v1.Promote), parameters)
	<-responseChannel

	appNamespace := fmt.Sprintf("%s-app", appName)
	jobs, _ := getJobsInNamespace(radixclient, appNamespace)

	assert.Equal(t, jobs[0].Spec.Promote.FromEnvironment, "origin")
	assert.Equal(t, jobs[0].Spec.Promote.ToEnvironment, "target")
	assert.Equal(t, jobs[0].Spec.Promote.DeploymentName, "a-deployment")
}

func TestIsDeployKeyValid(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _, _ := setupTest()
	commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithName("some-app").
		WithPublicKey("some-public-key").
		WithPrivateKey("some-private-key"))

	commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithName("some-app-missing-key").
		WithPublicKey("").
		WithPrivateKey(""))

	commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithName("some-app-missing-repository").
		WithCloneURL(""))

	// Tests
	t.Run("missing rr", func(t *testing.T) {
		responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deploykey-valid", "some-nonexisting-app"))
		response := <-responseChannel

		assert.Equal(t, http.StatusNotFound, response.Code)
	})

	t.Run("missing repository", func(t *testing.T) {
		responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deploykey-valid", "some-app-missing-repository"))
		response := <-responseChannel

		assert.Equal(t, http.StatusBadRequest, response.Code)

		errorResponse, _ := controllertest.GetErrorResponse(response)
		assert.Equal(t, "Clone URL is missing", errorResponse.Message)
	})

	t.Run("missing key", func(t *testing.T) {
		responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deploykey-valid", "some-app-missing-key"))
		response := <-responseChannel

		assert.Equal(t, http.StatusBadRequest, response.Code)

		errorResponse, _ := controllertest.GetErrorResponse(response)
		assert.Equal(t, "Deploy key is missing", errorResponse.Message)
	})

	t.Run("valid key", func(t *testing.T) {
		responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deploykey-valid", "some-app"))
		setStatusOfCloneJob(kubeclient, "some-app-app", true)

		response := <-responseChannel
		assert.Equal(t, http.StatusOK, response.Code)
	})

	t.Run("invalid key", func(t *testing.T) {
		responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deploykey-valid", "some-app"))
		setStatusOfCloneJob(kubeclient, "some-app-app", false)

		response := <-responseChannel
		assert.Equal(t, http.StatusBadRequest, response.Code)

		errorResponse, _ := controllertest.GetErrorResponse(response)
		assert.Equal(t, "Deploy key was invalid", errorResponse.Message)
	})
}

func TestDeleteApplication_ApplicationIsDeleted(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _ := setupTest()

	parameters := AnApplicationRegistration().
		withName("any-name").Build()

	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	<-responseChannel

	// Test
	responseChannel = controllerTestUtils.ExecuteRequest("DELETE", fmt.Sprintf("/api/v1/applications/%s", "any-non-existing"))
	response := <-responseChannel
	assert.Equal(t, http.StatusNotFound, response.Code)

	responseChannel = controllerTestUtils.ExecuteRequest("DELETE", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)

	// Application should no longer exist
	responseChannel = controllerTestUtils.ExecuteRequest("DELETE", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response = <-responseChannel
	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestGetApplication_WithAppAlias_ContainsAppAlias(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, client, radixclient, promclient := setupTest()
	utils.ApplyDeploymentWithSync(client, radixclient, promclient, commonTestUtils,
		builders.ARadixDeployment().
			WithAppName("any-app").
			WithEnvironment("prod").
			WithComponents(
				builders.NewDeployComponentBuilder().
					WithName("frontend").
					WithPort("http", 8080).
					WithPublicPort("http").
					WithDNSAppAlias(true),
				builders.NewDeployComponentBuilder().
					WithName("backend")))

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-app"))
	response := <-responseChannel

	application := applicationModels.Application{}
	controllertest.GetResponseBody(response, &application)

	assert.NotNil(t, application.AppAlias)
	assert.Equal(t, "frontend", application.AppAlias.ComponentName)
	assert.Equal(t, "prod", application.AppAlias.EnvironmentName)
	assert.Equal(t, fmt.Sprintf("%s.%s", "any-app", appAliasDNSZone), application.AppAlias.URL)
}

func TestListPipeline_ReturnesAvailablePipelines(t *testing.T) {
	supportedPipelines := jobPipeline.GetSupportedPipelines()

	// Setup
	commonTestUtils, controllerTestUtils, _, _, _ := setupTest()
	commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithName("some-app").
		WithPublicKey("some-public-key").
		WithPrivateKey("some-private-key"))

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/pipelines", "some-app"))
	response := <-responseChannel

	pipelines := make([]string, 0)
	controllertest.GetResponseBody(response, &pipelines)
	assert.Equal(t, len(supportedPipelines), len(pipelines))

}

func TestRegenerateDeployKey_WhenSecretProvided_GenerateNewDeployKeyAndSetSecret(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _ := setupTest()

	// Test
	appName := "any-name"
	origSharedSecret := "Orig shared secret"
	origDeployPublicKey := "Orig public key"
	parameters := AnApplicationRegistration().
		withName(appName).
		withRepository("https://github.com/Equinor/any-repo").
		withSharedSecret(origSharedSecret).
		withPrivateKey(origDeployPublicKey).
		withPublicKey("Orig private key").
		Build()

	appResponseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	<-appResponseChannel

	newSharedSecret := "new shared secret"
	regenerateParameters := AnRegenerateDeployKeyAndSecretDataBuilder().
		WithSharedSecret(newSharedSecret).
		Build()
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/regenerate-deploy-key", appName), regenerateParameters)
	response := <-responseChannel

	deployKeyAndSecret := applicationModels.DeployKeyAndSecret{}
	controllertest.GetResponseBody(response, &deployKeyAndSecret)
	assert.Equal(t, http.StatusOK, response.Code)
	assert.NotEqual(t, origDeployPublicKey, deployKeyAndSecret.PublicDeployKey)
	assert.True(t, strings.Contains(deployKeyAndSecret.PublicDeployKey, "ssh-rsa "))
	assert.Equal(t, newSharedSecret, deployKeyAndSecret.SharedSecret)
}

func TestRegenerateDeployKey_WhenSecretNotProvided_Fails(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _ := setupTest()

	// Test
	appName := "any-name"
	parameters := AnApplicationRegistration().
		withName(appName).
		withRepository("https://github.com/Equinor/any-repo").
		withSharedSecret("Orig shared secret").
		withPrivateKey("Orig public key").
		withPublicKey("Orig private key").
		Build()

	appResponseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	<-appResponseChannel

	newSharedSecret := ""
	regenerateParameters := AnRegenerateDeployKeyAndSecretDataBuilder().
		WithSharedSecret(newSharedSecret).
		Build()
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/regenerate-deploy-key", appName), regenerateParameters)
	response := <-responseChannel

	deployKeyAndSecret := applicationModels.DeployKeyAndSecret{}
	controllertest.GetResponseBody(response, &deployKeyAndSecret)
	assert.NotEqual(t, http.StatusOK, response.Code)
	assert.Empty(t, deployKeyAndSecret.PublicDeployKey)
	assert.Empty(t, deployKeyAndSecret.SharedSecret)
}

func TestRegenerateDeployKey_WhenApplicationNotExist_Fail(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _ := setupTest()

	// Test
	parameters := AnApplicationRegistration().
		withName("any-name").
		withRepository("https://github.com/Equinor/any-repo").
		Build()

	appResponseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	<-appResponseChannel

	regenerateParameters := AnRegenerateDeployKeyAndSecretDataBuilder().
		WithSharedSecret("new shared secret").
		Build()
	appName := "any-non-existing-name"
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/regenerate-deploy-key", appName), regenerateParameters)
	response := <-responseChannel

	deployKeyAndSecret := applicationModels.DeployKeyAndSecret{}
	controllertest.GetResponseBody(response, &deployKeyAndSecret)
	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Empty(t, deployKeyAndSecret.PublicDeployKey)
	assert.Empty(t, deployKeyAndSecret.SharedSecret)
}

func setStatusOfCloneJob(kubeclient kubernetes.Interface, appNamespace string, succeededStatus bool) {
	timeout := time.After(1 * time.Second)
	tick := time.Tick(200 * time.Millisecond)

	for {
		select {
		case <-timeout:
			return

		case <-tick:
			jobs, _ := kubeclient.BatchV1().Jobs(appNamespace).List(context.TODO(), metav1.ListOptions{})
			if len(jobs.Items) > 0 {
				job := jobs.Items[0]

				if succeededStatus {
					job.Status.Succeeded = int32(1)
				} else {
					job.Status.Failed = int32(1)
				}

				kubeclient.BatchV1().Jobs(appNamespace).Update(context.TODO(), &job, metav1.UpdateOptions{})
			}
		}
	}
}

func createRadixJob(commonTestUtils *commontest.Utils, appName, jobName string, started time.Time) {
	commonTestUtils.ApplyJob(
		builders.ARadixBuildDeployJob().
			WithAppName(appName).
			WithJobName(jobName).
			WithStatus(builders.NewJobStatusBuilder().
				WithCondition(v1.JobSucceeded).
				WithStarted(started.UTC()).
				WithSteps(
					builders.ACloneConfigStep().
						WithCondition(v1.JobSucceeded).
						WithStarted(started.UTC()).
						WithEnded(started.Add(time.Second*time.Duration(100))),
					builders.ARadixPipelineStep().
						WithCondition(v1.JobRunning).
						WithStarted(started.UTC()).
						WithEnded(started.Add(time.Second*time.Duration(100))))))
}

func getJobsInNamespace(radixclient *fake.Clientset, appNamespace string) ([]v1.RadixJob, error) {
	jobs, err := radixclient.RadixV1().RadixJobs(appNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return jobs.Items, nil
}

// RegenerateDeployKeyAndSecretDataBuilder Handles construction of DTO
type RegenerateDeployKeyAndSecretDataBuilder interface {
	WithSharedSecret(string) RegenerateDeployKeyAndSecretDataBuilder
	Build() *applicationModels.RegenerateDeployKeyAndSecretData
}

type regenerateDeployKeyAndSecretDataBuilder struct {
	sharedSecret string
}

func AnRegenerateDeployKeyAndSecretDataBuilder() RegenerateDeployKeyAndSecretDataBuilder {
	return &regenerateDeployKeyAndSecretDataBuilder{}
}

func (builder *regenerateDeployKeyAndSecretDataBuilder) WithSharedSecret(sharedSecret string) RegenerateDeployKeyAndSecretDataBuilder {
	builder.sharedSecret = sharedSecret
	return builder
}

func (builder *regenerateDeployKeyAndSecretDataBuilder) Build() *applicationModels.RegenerateDeployKeyAndSecretData {
	return &applicationModels.RegenerateDeployKeyAndSecretData{
		SharedSecret: builder.sharedSecret,
	}
}
