package jobs_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/equinor/radix-api/api/applications"
	authnmock "github.com/equinor/radix-api/api/utils/token/mock"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/golang/mock/gomock"
	kedav2 "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	"github.com/stretchr/testify/require"
	secretsstorevclient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"

	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"

	"github.com/stretchr/testify/assert"

	certclientfake "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned/fake"
	. "github.com/equinor/radix-api/api/jobs"
	jobmodels "github.com/equinor/radix-api/api/jobs/models"
	controllertest "github.com/equinor/radix-api/api/test"
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

func setupTest(t *testing.T) (*commontest.Utils, *controllertest.Utils, kubernetes.Interface, radixclient.Interface, kedav2.Interface, secretsstorevclient.Interface, *certclientfake.Clientset) {
	// Setup
	kubeclient := kubefake.NewSimpleClientset()
	radixclient := fake.NewSimpleClientset()
	kedaClient := kedafake.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	certClient := certclientfake.NewSimpleClientset()

	// commonTestUtils is used for creating CRDs
	commonTestUtils := commontest.NewTestUtils(kubeclient, radixclient, kedaClient, secretproviderclient)

	// controllerTestUtils is used for issuing HTTP request and processing responses
	mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
	mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).AnyTimes().Return(controllertest.NewTestPrincipal(true), nil)
	controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, kedaClient, secretproviderclient, certClient, mockValidator, NewJobController())

	return &commonTestUtils, &controllerTestUtils, kubeclient, radixclient, kedaClient, secretproviderclient, certClient
}

func TestGetApplicationJob(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, client, radixclient, _, _, _ := setupTest(t)

	_, err := commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithName(anyAppName).
		WithCloneURL(anyCloneURL))
	require.NoError(t, err)

	jobParameters := &jobmodels.JobParameters{
		Branch:                anyBranch,
		CommitID:              anyPushCommitID,
		PushImage:             true,
		TriggeredBy:           anyUser,
		OverrideUseBuildCache: pointers.Ptr(true),
	}

	anyPipeline, _ := pipeline.GetPipelineFromName(anyPipelineName)
	anyPipelineTagName := "latestPipelineImageTag"
	anyTektonTagName := "latestTektonImageTag"
	jobSummary, _ := applications.HandleStartPipelineJob(context.Background(), radixclient, anyAppName, anyPipelineTagName, anyTektonTagName, anyPipeline, jobParameters)
	_, err = createPipelinePod(client, builders.GetAppNamespace(anyAppName), jobSummary.Name)
	require.NoError(t, err)

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/jobs/%s", anyAppName, jobSummary.Name))
	response := <-responseChannel

	job := jobmodels.Job{}
	err = controllertest.GetResponseBody(response, &job)
	require.NoError(t, err)
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
	_, err = addInitStepsToPipelinePod(client, builders.GetAppNamespace(anyAppName), jobSummary.Name, internalStep, cloneStep)
	require.NoError(t, err)
	_, err = addStepToPipelinePod(client, builders.GetAppNamespace(anyAppName), jobSummary.Name, pipelineStep)
	require.NoError(t, err)

	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/jobs/%s", anyAppName, jobSummary.Name))
	response = <-responseChannel

	job = jobmodels.Job{}
	err = controllertest.GetResponseBody(response, &job)
	require.NoError(t, err)
	assert.Equal(t, jobSummary.Name, job.Name)
	assert.Equal(t, anyBranch, job.Branch)
	assert.Equal(t, anyPushCommitID, job.CommitID)
	assert.Equal(t, anyUser, job.TriggeredBy)
	assert.Equal(t, string(anyPipeline.Type), job.Pipeline)
	assert.NotNil(t, jobSummary.OverrideUseBuildCache)
	assert.True(t, *jobSummary.OverrideUseBuildCache)
}

func TestGetApplicationJob_RadixJobSpecExists(t *testing.T) {
	anyAppName := "any-app"
	anyJobName := "any-job"

	// Setup
	commonTestUtils, controllerTestUtils, _, _, _, _, _ := setupTest(t)
	job, _ := commonTestUtils.ApplyJob(builders.AStartedBuildDeployJob().WithAppName(anyAppName).WithJobName(anyJobName))

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/jobs/%s", anyAppName, anyJobName))
	response := <-responseChannel

	jobSummary := jobmodels.Job{}
	err := controllertest.GetResponseBody(response, &jobSummary)
	require.NoError(t, err)
	assert.Equal(t, job.Name, jobSummary.Name)
	assert.Equal(t, job.Spec.Build.Branch, jobSummary.Branch)
	assert.Equal(t, string(job.Spec.PipeLineType), jobSummary.Pipeline)
	assert.Equal(t, len(job.Status.Steps), len(jobSummary.Steps))

}

func TestGetPipelineJobLogsError(t *testing.T) {
	commonTestUtils, controllerTestUtils, _, _, _, _, _ := setupTest(t)

	t.Run("job doesn't exist", func(t *testing.T) {
		aJobName := "aJobName"
		cloneConfigStepName := "clone-config"
		responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/jobs/%s/logs/%s", anyAppName, aJobName, cloneConfigStepName))
		response := <-responseChannel

		httpErr, err := controllertest.GetErrorResponse(response)
		require.NoError(t, err)
		assert.NotNil(t, httpErr)
		assert.Equal(t, controllertest.AppNotFoundErrorMsg(anyAppName), httpErr.Message)

		_, err = commonTestUtils.ApplyApplication(builders.ARadixApplication().
			WithAppName(anyAppName))
		require.NoError(t, err)

		responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/jobs/%s/logs/%s", anyAppName, aJobName, cloneConfigStepName))
		response = <-responseChannel

		pipelineNotFoundError := jobmodels.PipelineNotFoundError(anyAppName, aJobName)
		err, _ = controllertest.GetErrorResponse(response)
		assert.NotNil(t, err)
		assert.Equal(t, pipelineNotFoundError.Error(), err.Error())
	})
}

func createPipelinePod(kubeclient kubernetes.Interface, namespace, jobName string) (*corev1.Pod, error) {
	podSpec := getPodSpecForAPipelineJob(jobName)
	return kubeclient.CoreV1().Pods(namespace).Create(context.Background(), podSpec, metav1.CreateOptions{})
}

func addInitStepsToPipelinePod(kubeclient kubernetes.Interface, namespace, jobName string, initSteps ...corev1.ContainerStatus) (*corev1.Pod, error) {
	pipelinePod, _ := kubeclient.CoreV1().Pods(namespace).Get(context.Background(), jobName, metav1.GetOptions{})
	podStatus := pipelinePod.Status
	podStatus.InitContainerStatuses = append(podStatus.InitContainerStatuses, initSteps...)
	pipelinePod.Status = podStatus
	return kubeclient.CoreV1().Pods(namespace).Update(context.Background(), pipelinePod, metav1.UpdateOptions{})
}

func addStepToPipelinePod(kubeclient kubernetes.Interface, namespace, jobName string, jobStep corev1.ContainerStatus) (*corev1.Pod, error) {
	pipelinePod, _ := kubeclient.CoreV1().Pods(namespace).Get(context.Background(), jobName, metav1.GetOptions{})
	podStatus := pipelinePod.Status
	podStatus.ContainerStatuses = append(podStatus.ContainerStatuses, jobStep)
	pipelinePod.Status = podStatus
	return kubeclient.CoreV1().Pods(namespace).Update(context.Background(), pipelinePod, metav1.UpdateOptions{})
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
