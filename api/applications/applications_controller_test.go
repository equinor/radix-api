package applications

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	applicationModels "github.com/statoil/radix-api/api/applications/models"
	jobModels "github.com/statoil/radix-api/api/jobs/models"
	controllertest "github.com/statoil/radix-api/api/test"
	commontest "github.com/statoil/radix-operator/pkg/apis/test"
	builders "github.com/statoil/radix-operator/pkg/apis/utils"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	"github.com/statoil/radix-operator/pkg/client/clientset/versioned/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func setupTest() (*commontest.Utils, *controllertest.Utils, kubernetes.Interface, radixclient.Interface) {
	// Setup
	kubeclient := kubefake.NewSimpleClientset()
	radixclient := fake.NewSimpleClientset()

	// commonTestUtils is used for creating CRDs
	commonTestUtils := commontest.NewTestUtils(kubeclient, radixclient)

	// controllerTestUtils is used for issuing HTTP request and processing responses
	controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, NewApplicationController())

	return &commonTestUtils, &controllerTestUtils, kubeclient, radixclient
}

func TestGetApplications_WithFilterOnSSHRepo_Filter(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _ := setupTest()
	commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithCloneURL("git@github.com:Equinor/my-app.git"))

	// Test
	t.Run("matching repo", func(t *testing.T) {
		responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications?sshRepo=%s", url.QueryEscape("git@github.com:Equinor/my-app.git")))
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationRegistration, 0)
		controllertest.GetResponseBody(response, &applications)
		assert.Equal(t, 1, len(applications))
	})

	t.Run("unmatching repo", func(t *testing.T) {
		responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications?sshRepo=%s", url.QueryEscape("git@github.com:Equinor/my-app2.git")))
		response := <-responseChannel

		applications := make([]*applicationModels.ApplicationRegistration, 0)
		controllertest.GetResponseBody(response, &applications)
		assert.Equal(t, 0, len(applications))
	})

	t.Run("no filter", func(t *testing.T) {
		responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/applications")
		response := <-responseChannel

		applications := make([]*applicationModels.ApplicationRegistration, 0)
		controllertest.GetResponseBody(response, &applications)
		assert.Equal(t, 1, len(applications))
	})
}

func TestCreateApplication_NoName_ValidationError(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _ := setupTest()

	// Test
	parameters := ABuilder().withName("").BuildApplicationRegistration()
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusUnprocessableEntity, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	assert.Equal(t, "Error: app name is required", errorResponse.Message)
}

func TestCreateApplication_WhenRepoIsNotSet_DoNotGenerateDeployKey(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _ := setupTest()

	// Test
	parameters := ABuilder().withRepository("").BuildApplicationRegistration()
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	response := <-responseChannel

	application := applicationModels.ApplicationRegistration{}
	controllertest.GetResponseBody(response, &application)
	assert.Equal(t, "", application.PublicKey)
}

func TestCreateApplication_WhenRepoIsSetAnDeployKeyIsNot_GenerateDeployKey(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _ := setupTest()

	// Test
	parameters := ABuilder().
		withName("any-name-1").
		withRepository("https://github.com/Equinor/any-repo").
		BuildApplicationRegistration()
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	response := <-responseChannel

	application := applicationModels.ApplicationRegistration{}
	controllertest.GetResponseBody(response, &application)
	assert.NotEmpty(t, application.PublicKey)
}

func TestCreateApplication_WhenDeployKeyIsSet_DoNotGenerateDeployKey(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _ := setupTest()

	// Test
	parameters := ABuilder().
		withName("any-name-2").
		withRepository("https://github.com/Equinor/any-repo").
		withPublicKey("Any public key").
		BuildApplicationRegistration()
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	response := <-responseChannel

	application := applicationModels.ApplicationRegistration{}
	controllertest.GetResponseBody(response, &application)
	assert.Equal(t, "Any public key", application.PublicKey)

}

func TestCreateApplication_DuplicateRepo_ShouldFailAsWeCannotHandleThatSituation(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _ := setupTest()

	parameters := ABuilder().
		withName("any-name").
		withRepository("https://github.com/Equinor/any-repo").
		BuildApplicationRegistration()
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	<-responseChannel

	// Test
	parameters = ABuilder().
		withName("any-other-name").
		withRepository("https://github.com/Equinor/any-repo").
		BuildApplicationRegistration()
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusUnprocessableEntity, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	assert.Equal(t, "Error: Repository is in use by any-name", errorResponse.Message)
}

func TestGetApplication_AllFieldsAreSet(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _ := setupTest()

	parameters := ABuilder().
		withName("any-name").
		withRepository("https://github.com/Equinor/any-repo").
		withSharedSecret("Any secret").
		withAdGroups([]string{"a6a3b81b-34gd-sfsf-saf2-7986371ea35f"}).BuildApplicationRegistration()

	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	<-responseChannel

	// Test
	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response := <-responseChannel

	application := applicationModels.ApplicationRegistration{}
	controllertest.GetResponseBody(response, &application)

	assert.Equal(t, "https://github.com/Equinor/any-repo", application.Repository)
	assert.Equal(t, "Any secret", application.SharedSecret)
	assert.Equal(t, []string{"a6a3b81b-34gd-sfsf-saf2-7986371ea35f"}, application.AdGroups)

}

func TestUpdateApplication_DuplicateRepo_ShouldFailAsWeCannotHandleThatSituation(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _ := setupTest()

	parameters := ABuilder().
		withName("any-name").
		withRepository("https://github.com/Equinor/any-repo").
		BuildApplicationRegistration()

	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	<-responseChannel

	parameters = ABuilder().
		withName("any-other-name").
		withRepository("https://github.com/Equinor/any-other-repo").
		BuildApplicationRegistration()

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	<-responseChannel

	// Test
	parameters = ABuilder().
		withName("any-other-name").
		withRepository("https://github.com/Equinor/any-repo").
		BuildApplicationRegistration()

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "any-other-name"), parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusUnprocessableEntity, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	assert.Equal(t, "Error: Repository is in use by any-name", errorResponse.Message)
}

func TestUpdateApplication_MismatchingNameOrNotExists_ShouldFailAsIllegalOperation(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _ := setupTest()

	parameters := ABuilder().withName("any-name").BuildApplicationRegistration()
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	<-responseChannel

	// Test
	parameters = ABuilder().withName("any-name").BuildApplicationRegistration()
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "another-name"), parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusUnprocessableEntity, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	assert.Equal(t, "App name another-name does not correspond with application name any-name", errorResponse.Message)

	parameters = ABuilder().withName("another-name").BuildApplicationRegistration()
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "another-name"), parameters)
	response = <-responseChannel
	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestUpdateApplication_AbleToSetAnySpecField(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _ := setupTest()

	builder := ABuilder().
		withName("any-name").
		withRepository("https://github.com/Equinor/a-repo").
		withSharedSecret("").
		withPublicKey("")
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", builder.BuildApplicationRegistration())
	<-responseChannel

	// Test
	builder = builder.
		withRepository("https://github.com/Equinor/any-repo")

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "any-name"), builder.BuildApplicationRegistration())
	response := <-responseChannel

	application := applicationModels.ApplicationRegistration{}
	controllertest.GetResponseBody(response, &application)
	assert.Equal(t, "https://github.com/Equinor/any-repo", application.Repository)

	builder = builder.
		withSharedSecret("Any shared secret")

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "any-name"), builder.BuildApplicationRegistration())
	response = <-responseChannel
	controllertest.GetResponseBody(response, &application)
	assert.Equal(t, "Any shared secret", application.SharedSecret)

	builder = builder.
		withPublicKey("Any public key")

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "any-name"), builder.BuildApplicationRegistration())
	response = <-responseChannel
	controllertest.GetResponseBody(response, &application)
	assert.Equal(t, "Any public key", application.PublicKey)

}

func TestHandleTriggerPipeline_ExistingAndNonExistingApplication_JobIsCreatedForExisting(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _ := setupTest()

	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", ABuilder().withName("any-app").BuildApplicationRegistration())
	<-responseChannel

	// Test
	const pushCommitID = "4faca8595c5283a9d0f17a623b9255a0d9866a2e"

	parameters := applicationModels.PipelineParameters{Branch: "master", CommitID: pushCommitID}
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/pipelines/%s", " ", jobModels.BuildDeploy.String()), parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusUnprocessableEntity, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	assert.Equal(t, "App name and branch are required", errorResponse.Message)

	parameters = applicationModels.PipelineParameters{Branch: "", CommitID: pushCommitID}
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/pipelines/%s", "any-app", jobModels.BuildDeploy.String()), parameters)
	response = <-responseChannel

	assert.Equal(t, http.StatusUnprocessableEntity, response.Code)
	errorResponse, _ = controllertest.GetErrorResponse(response)
	assert.Equal(t, "App name and branch are required", errorResponse.Message)

	parameters = applicationModels.PipelineParameters{Branch: "master", CommitID: pushCommitID}
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/pipelines/%s", "any-app", jobModels.BuildDeploy.String()), parameters)
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)

	jobSummary := jobModels.JobSummary{}
	controllertest.GetResponseBody(response, &jobSummary)
	assert.Equal(t, "any-app", jobSummary.AppName)
	assert.Equal(t, "master", jobSummary.Branch)
	assert.Equal(t, pushCommitID, jobSummary.CommitID)
}

func TestIsDeployKeyValid(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _ := setupTest()
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

		assert.Equal(t, http.StatusUnprocessableEntity, response.Code)

		errorResponse, _ := controllertest.GetErrorResponse(response)
		assert.Equal(t, "Clone URL is missing", errorResponse.Message)
	})

	t.Run("missing key", func(t *testing.T) {
		responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deploykey-valid", "some-app-missing-key"))
		response := <-responseChannel

		assert.Equal(t, http.StatusUnprocessableEntity, response.Code)

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
		assert.Equal(t, http.StatusUnprocessableEntity, response.Code)

		errorResponse, _ := controllertest.GetErrorResponse(response)
		assert.Equal(t, "Deploy key was invalid", errorResponse.Message)
	})
}

func setStatusOfCloneJob(kubeclient kubernetes.Interface, appNamespace string, succeededStatus bool) {
	timeout := time.After(1 * time.Second)
	tick := time.Tick(200 * time.Millisecond)

	for {
		select {
		case <-timeout:
			return

		case <-tick:
			jobs, _ := kubeclient.BatchV1().Jobs(appNamespace).List(metav1.ListOptions{})
			if len(jobs.Items) > 0 {
				job := jobs.Items[0]

				if succeededStatus {
					job.Status.Succeeded = int32(1)
				} else {
					job.Status.Failed = int32(1)
				}

				kubeclient.BatchV1().Jobs(appNamespace).Update(&job)
			}
		}
	}
}
