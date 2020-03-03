package jobs_test

import (
	"fmt"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"

	"github.com/stretchr/testify/assert"

	. "github.com/equinor/radix-api/api/jobs"
	jobModels "github.com/equinor/radix-api/api/jobs/models"
	controllertest "github.com/equinor/radix-api/api/test"
	"github.com/equinor/radix-api/models"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	builders "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
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
	anyPipelineName = string(v1.BuildDeploy)
	anyUser         = "a_user@equinor.com"
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

func TestIsBefore(t *testing.T) {
	job1 := jobModels.JobSummary{}
	job2 := jobModels.JobSummary{}

	job1.Created = ""
	job2.Created = ""
	assert.False(t, IsBefore(&job1, &job2))

	job1.Created = "2019-08-26T12:56:48Z"
	job2.Created = ""
	assert.True(t, IsBefore(&job1, &job2))

	job1.Created = "2019-08-26T12:56:48Z"
	job2.Created = "2019-08-26T12:56:49Z"
	assert.True(t, IsBefore(&job1, &job2))

	job1.Created = "2019-08-26T12:56:48Z"
	job2.Created = "2019-08-26T12:56:48Z"
	job1.Started = "2019-08-26T12:56:51Z"
	job2.Started = "2019-08-26T12:56:52Z"
	assert.True(t, IsBefore(&job1, &job2))

	job1.Created = "2019-08-26T12:56:48Z"
	job2.Created = "2019-08-26T12:56:48Z"
	job1.Started = ""
	job2.Started = "2019-08-26T12:56:52Z"
	assert.False(t, IsBefore(&job1, &job2))

}

func TestGetApplicationJob(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, client, radixclient := setupTest()

	commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithName(anyAppName).
		WithCloneURL(anyCloneURL))

	jobParameters := &jobModels.JobParameters{
		Branch:      anyBranch,
		CommitID:    anyPushCommitID,
		PushImage:   true,
		TriggeredBy: anyUser,
	}

	handler := Init(models.NewAccounts(client, radixclient, client, radixclient, "", models.Impersonation{}))

	anyPipeline, _ := pipeline.GetPipelineFromName(anyPipelineName)
	jobSummary, _ := handler.HandleStartPipelineJob(anyAppName, anyPipeline, jobParameters)
	createPipelinePod(client, builders.GetAppNamespace(anyAppName), jobSummary.Name)

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/jobs/%s", anyAppName, jobSummary.Name))
	response := <-responseChannel

	job := jobModels.Job{}
	controllertest.GetResponseBody(response, &job)
	assert.Equal(t, jobSummary.Name, job.Name)
	assert.Equal(t, anyBranch, job.Branch)
	assert.Equal(t, anyPushCommitID, job.CommitID)
	assert.Equal(t, anyUser, job.TriggeredBy)
	assert.Equal(t, string(anyPipeline.Type), job.Pipeline)
	assert.Empty(t, job.Steps)

	internalStep := corev1.ContainerStatus{Name: fmt.Sprintf("%sAnyStep", git.InternalContainerPrefix), State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{}}}
	cloneStep := corev1.ContainerStatus{Name: git.CloneContainerName, State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{}}}
	pipelineStep := corev1.ContainerStatus{Name: "radix-pipeline", State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{}}}

	// Emulate a running job with two steps
	addInitStepsToPipelinePod(client, builders.GetAppNamespace(anyAppName), jobSummary.Name, internalStep, cloneStep)
	addStepToPipelinePod(client, builders.GetAppNamespace(anyAppName), jobSummary.Name, pipelineStep)

	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/jobs/%s", anyAppName, jobSummary.Name))
	response = <-responseChannel

	job = jobModels.Job{}
	controllertest.GetResponseBody(response, &job)
	assert.Equal(t, jobSummary.Name, job.Name)
	assert.Equal(t, anyBranch, job.Branch)
	assert.Equal(t, anyPushCommitID, job.CommitID)
	assert.Equal(t, anyUser, job.TriggeredBy)
	assert.Equal(t, string(anyPipeline.Type), job.Pipeline)

}

func TestGetApplicationJob_RadixJobSpecExists(t *testing.T) {
	anyAppName := "any-app"
	anyJobName := "any-job"

	// Setup
	commonTestUtils, controllerTestUtils, _, _ := setupTest()
	job, _ := commonTestUtils.ApplyJob(builders.AStartedBuildDeployJob().WithAppName(anyAppName).WithJobName(anyJobName))

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/jobs/%s", anyAppName, anyJobName))
	response := <-responseChannel

	jobSummary := jobModels.Job{}
	controllertest.GetResponseBody(response, &jobSummary)
	assert.Equal(t, job.Name, jobSummary.Name)
	assert.Equal(t, job.Spec.Build.Branch, jobSummary.Branch)
	assert.Equal(t, string(job.Spec.PipeLineType), jobSummary.Pipeline)
	assert.Equal(t, len(job.Status.Steps), len(jobSummary.Steps))

}

func TestGetPipelineJobLogsError(t *testing.T) {
	commonTestUtils, controllerTestUtils, _, _ := setupTest()

	t.Run("job doesn't exist", func(t *testing.T) {
		aJobName := "aJobName"
		responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/jobs/%s/logs", anyAppName, aJobName))
		response := <-responseChannel

		err, _ := controllertest.GetErrorResponse(response)
		assert.NotNil(t, err)
		assert.Equal(t, controllertest.AppNotFoundErrorMsg(anyAppName), err.Message)

		commonTestUtils.ApplyApplication(builders.ARadixApplication().
			WithAppName(anyAppName))

		responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/jobs/%s/logs", anyAppName, aJobName))
		response = <-responseChannel

		pipelineNotFoundError := jobModels.PipelineNotFoundError(anyAppName, aJobName)
		err, _ = controllertest.GetErrorResponse(response)
		assert.NotNil(t, err)
		assert.Equal(t, pipelineNotFoundError.Error(), err.Error())
	})
}

func createPipelinePod(kubeclient kubernetes.Interface, namespace, jobName string) {
	podSpec := getPodSpecForAPipelineJob(jobName)
	kubeclient.CoreV1().Pods(namespace).Create(podSpec)
}

func addInitStepsToPipelinePod(kubeclient kubernetes.Interface, namespace, jobName string, initSteps ...corev1.ContainerStatus) {
	pipelinePod, _ := kubeclient.CoreV1().Pods(namespace).Get(jobName, metav1.GetOptions{})
	podStatus := pipelinePod.Status
	podStatus.InitContainerStatuses = append(podStatus.InitContainerStatuses, initSteps...)
	pipelinePod.Status = podStatus
	kubeclient.CoreV1().Pods(namespace).Update(pipelinePod)
}

func addStepToPipelinePod(kubeclient kubernetes.Interface, namespace, jobName string, jobStep corev1.ContainerStatus) {
	pipelinePod, _ := kubeclient.CoreV1().Pods(namespace).Get(jobName, metav1.GetOptions{})
	podStatus := pipelinePod.Status
	podStatus.ContainerStatuses = append(podStatus.ContainerStatuses, jobStep)
	pipelinePod.Status = podStatus
	kubeclient.CoreV1().Pods(namespace).Update(pipelinePod)
}

func getPodSpecForAPipelineJob(jobName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
			Labels: map[string]string{
				"job-name": jobName,
			},
		},
	}
}
