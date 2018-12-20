package jobs_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/statoil/radix-api/api/jobs"
	jobModels "github.com/statoil/radix-api/api/jobs/models"
	controllertest "github.com/statoil/radix-api/api/test"
	"github.com/statoil/radix-operator/pkg/apis/kube"
	commontest "github.com/statoil/radix-operator/pkg/apis/test"
	builders "github.com/statoil/radix-operator/pkg/apis/utils"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	"github.com/statoil/radix-operator/pkg/client/clientset/versioned/fake"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

const (
	anyAppName      = "any-app"
	anyCloneURL     = "git@github.com:Equinor/any-app.git"
	anyBranch       = "master"
	anyPushCommitID = "4faca8595c5283a9d0f17a623b9255a0d9866a2e"
	anyPipeline     = jobModels.BuildDeploy
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
	commonTestUtils, controllerTestUtils, client, radixclient := setupTest()

	commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithName(anyAppName).
		WithCloneURL(anyCloneURL))

	jobParameters := &jobModels.JobParameters{
		Branch:   anyBranch,
		CommitID: anyPushCommitID,
	}

	handler := Init(client, radixclient)
	jobSummary, _ := handler.HandleStartPipelineJob(anyAppName, anyCloneURL, anyPipeline, jobParameters)
	createPipelinePod(client, builders.GetAppNamespace(anyAppName), jobSummary.Name)

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/jobs/%s", anyAppName, jobSummary.Name))
	response := <-responseChannel

	job := jobModels.Job{}
	controllertest.GetResponseBody(response, &job)
	assert.Equal(t, jobSummary.Name, job.Name)
	assert.Equal(t, anyBranch, job.Branch)
	assert.Equal(t, anyPushCommitID, job.CommitID)
	assert.Equal(t, anyPipeline.String(), job.Pipeline)
	assert.Empty(t, job.Steps)

	cloneStep := corev1.ContainerStatus{Name: "clone", State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{}}}
	addStepToPipelinePod(client, builders.GetAppNamespace(anyAppName), jobSummary.Name, cloneStep)

	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/jobs/%s", anyAppName, jobSummary.Name))
	response = <-responseChannel

	job = jobModels.Job{}
	controllertest.GetResponseBody(response, &job)
	assert.Equal(t, jobSummary.Name, job.Name)
	assert.Equal(t, anyBranch, job.Branch)
	assert.Equal(t, anyPushCommitID, job.CommitID)
	assert.Equal(t, anyPipeline.String(), job.Pipeline)
	assert.NotEmpty(t, job.Steps)

	assert.

}

func TestGetPipelineJobLogsError(t *testing.T) {
	_, controllerTestUtils, _, _ := setupTest()

	t.Run("job doesn't exist", func(t *testing.T) {
		aJobName := "aJobName"
		responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/jobs/%s/logs", anyAppName, aJobName))
		response := <-responseChannel

		pipelineNotFoundError := jobModels.PipelineNotFoundError(anyAppName, aJobName)
		err, _ := controllertest.GetErrorResponse(response)
		assert.NotNil(t, err)
		assert.Equal(t, pipelineNotFoundError.Error(), err.Error())
	})
}

func createPipelinePod(kubeclient kubernetes.Interface, namespace, jobName string) {
	podSpec := getPodSpecForAPipelineJob(jobName)
	kubeclient.CoreV1().Pods(namespace).Create(podSpec)
}

func addStepToPipelinePod(kubeclient kubernetes.Interface, namespace, jobName string, jobStep corev1.ContainerStatus) {
	pipelinePod, _ := kubeclient.CoreV1().Pods(namespace).Get(jobName, metav1.GetOptions{})
	podStatus := corev1.PodStatus{
		ContainerStatuses: []corev1.ContainerStatus{
			jobStep,
		},
	}

	pipelinePod.Status = podStatus
	kubeclient.CoreV1().Pods(namespace).Update(pipelinePod)
}

func getPodSpecForAPipelineJob(jobName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
			Labels: map[string]string{
				kube.RadixJobNameLabel: jobName,
			},
		},
	}
}
