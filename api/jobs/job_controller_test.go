package jobs

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	jobModels "github.com/statoil/radix-api/api/jobs/models"
	controllertest "github.com/statoil/radix-api/api/test"
	commontest "github.com/statoil/radix-operator/pkg/apis/test"
	builders "github.com/statoil/radix-operator/pkg/apis/utils"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	"github.com/statoil/radix-operator/pkg/client/clientset/versioned/fake"
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
	controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, NewJobController())

	return &commonTestUtils, &controllerTestUtils, kubeclient, radixclient
}

func TestGetApplicationJob(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, client, _ := setupTest()

	const anyAppName = "any-app"
	const anyCloneURL = "git@github.com:Equinor/any-app.git"
	const anyBranch = "master"
	const anyPushCommitID = "4faca8595c5283a9d0f17a623b9255a0d9866a2e"
	const anyPipeline = jobModels.BuildDeploy

	commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithName(anyAppName).
		WithCloneURL(anyCloneURL))

	jobParameters := &jobModels.JobParameters{
		Branch:   anyBranch,
		CommitID: anyPushCommitID,
	}

	// Test
	t.Run("job started ok", func(t *testing.T) {
		jobSummary, _ := HandleStartPipelineJob(client, anyAppName, anyCloneURL, anyPipeline, jobParameters)
		responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/jobs/%s", anyAppName, jobSummary.Name))
		response := <-responseChannel

		job := jobModels.Job{}
		controllertest.GetResponseBody(response, &job)
		assert.Equal(t, jobSummary.Name, job.Name)
		assert.Equal(t, anyBranch, job.Branch)
		assert.Equal(t, anyPushCommitID, job.CommitID)
		assert.Equal(t, anyPipeline.String(), job.Pipeline)
	})

}
