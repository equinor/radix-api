package jobs

import (
	"fmt"
	"sort"
	"strings"

	jobModels "github.com/equinor/radix-api/api/jobs/models"
	"github.com/equinor/radix-api/api/utils"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// JIRA : https://equinor.atlassian.net/browse/RA-725
// TODO : Remove this entire file and all references to it when radixjobs has been in prod for a while
func (jh JobHandler) getApplicationJobsLegacy(appName string) ([]*jobModels.JobSummary, error) {
	jobList, err := jh.getJobsLegacy(appName)
	if err != nil {
		return nil, err
	}

	// Sort jobs descending
	sort.Slice(jobList.Items, func(i, j int) bool {
		return jobList.Items[j].Status.StartTime.Before(jobList.Items[i].Status.StartTime)
	})

	jobEnvironmentsMap, err := jh.getJobEnvironmentMapLegacy(appName)
	if err != nil {
		return nil, err
	}

	jobs := make([]*jobModels.JobSummary, len(jobList.Items))
	for i, job := range jobList.Items {
		jobSummary, err := jh.getJobSummaryWithDeploymentLegacy(appName, &job, jobEnvironmentsMap)
		if err != nil {
			return nil, err
		}

		jobs[i] = jobSummary
	}

	return jobs, nil
}

// TODO : Remove when radixjobs has been in prod for a while
func (jh JobHandler) getApplicationJobLegacy(appName, jobName string) (*jobModels.Job, error) {
	job, err := jh.userAccount.Client.BatchV1().Jobs(crdUtils.GetAppNamespace(appName)).Get(jobName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return nil, jobModels.PipelineNotFoundError(appName, jobName)
	}
	if err != nil {
		return nil, err
	}

	if !strings.EqualFold(job.Labels[kube.RadixJobTypeLabel], RadixJobTypeJob) {
		return nil, utils.ValidationError("Radix Application Job", "Job was not of expected type")
	}

	steps, err := jh.getJobStepsLegacy(appName, job)
	if err != nil {
		return nil, err
	}

	jobDeployments, err := jh.deploy.GetDeploymentsForJob(appName, jobName)
	if err != nil {
		return nil, err
	}

	jobComponents, err := jh.getJobComponents(appName, jobName)
	if err != nil {
		return nil, err
	}

	return jobModels.GetJob(job, steps, jobDeployments, jobComponents), nil
}

// TODO : Remove when radixjobs has been in prod for a while
func (jh JobHandler) getJobStepsLegacy(appName string, job *batchv1.Job) ([]jobModels.Step, error) {
	steps := []jobModels.Step{}
	jobName := job.GetName()

	pipelinePod, err := jh.getPipelinePod(appName, jobName)
	if err != nil {
		return nil, err
	} else if pipelinePod == nil {
		return steps, nil
	}

	if len(pipelinePod.Status.ContainerStatuses) == 0 {
		return steps, nil
	}

	pipelineType, _ := pipeline.GetPipelineFromName(job.Labels["radix-pipeline"])

	switch pipelineType.Type {
	case v1.Build, v1.BuildDeploy:
		return jh.getJobStepsBuildPipelineLegacy(appName, pipelinePod, job)
	case v1.Promote:
		return jh.getJobStepsPromotePipelineLegacy(appName, pipelinePod, job)
	}

	return steps, nil
}

// TODO : Remove when radixjobs has been in prod for a while
func (jh JobHandler) getJobStepsBuildPipelineLegacy(appName string, pipelinePod *corev1.Pod, job *batchv1.Job) ([]jobModels.Step, error) {
	steps := []jobModels.Step{}
	if len(pipelinePod.Status.InitContainerStatuses) == 0 {
		return steps, nil
	}

	pipelineJobStep := getPipelineJobStep(pipelinePod)
	cloneContainerStatus := getCloneContainerStatus(pipelinePod)
	if cloneContainerStatus == nil {
		return steps, nil
	}

	// Clone of radix config should be represented
	pipelineCloneStep := getJobStep(pipelinePod.GetName(), cloneContainerStatus)
	pipelineCloneStep.Name = "clone-config"

	jobStepsLabelSelector := fmt.Sprintf("%s=%s, %s!=%s", kube.RadixImageTagLabel, job.Labels[kube.RadixImageTagLabel], kube.RadixJobTypeLabel, RadixJobTypeJob)

	jobStepList, err := jh.userAccount.Client.BatchV1().Jobs(crdUtils.GetAppNamespace(appName)).List(metav1.ListOptions{
		LabelSelector: jobStepsLabelSelector,
	})

	if err != nil {
		return nil, err
	}

	// pipeline coordinator
	steps = append(steps, pipelineCloneStep, pipelineJobStep)
	for _, jobStep := range jobStepList.Items {
		jobStepPod, err := jh.userAccount.Client.CoreV1().Pods(crdUtils.GetAppNamespace(appName)).List(metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", "job-name", jobStep.Name),
		})

		if err != nil {
			return nil, err
		}

		if len(jobStepPod.Items) == 0 {
			continue
		}

		pod := jobStepPod.Items[0]
		for _, containerStatus := range pod.Status.InitContainerStatuses {
			if strings.HasPrefix(containerStatus.Name, git.InternalContainerPrefix) {
				continue
			}

			steps = append(steps, getJobStep(pod.GetName(), &containerStatus))
		}

		for _, containerStatus := range pod.Status.ContainerStatuses {
			steps = append(steps, getJobStep(pod.GetName(), &containerStatus))
		}
	}

	return steps, nil
}

func (jh JobHandler) getJobStepsPromotePipelineLegacy(appName string, pipelinePod *corev1.Pod, job *batchv1.Job) ([]jobModels.Step, error) {
	steps := []jobModels.Step{}
	pipelineJobStep := getJobStep(pipelinePod.GetName(), &pipelinePod.Status.ContainerStatuses[0])
	steps = append(steps, pipelineJobStep)
	return steps, nil
}

// TODO : Remove when radixjobs has been in prod for a while
// GetJobSummaryWithDeployment Used to get job summary from a kubernetes job
func (jh JobHandler) getJobSummaryWithDeploymentLegacy(appName string, job *batchv1.Job, jobEnvironmentsMap map[string][]string) (*jobModels.JobSummary, error) {
	jobSummary := jobModels.GetJobSummary(job)
	jobSummary.Environments = jobEnvironmentsMap[job.Name]
	return jobSummary, nil
}

// TODO : Remove when radixjobs has been in prod for a while
func (jh JobHandler) getJobEnvironmentMapLegacy(appName string) (map[string][]string, error) {
	allDeployments, err := jh.deploy.GetDeploymentsForApplication(appName, false)
	if err != nil {
		return nil, err
	}

	jobEnvironmentsMap := make(map[string][]string)
	for _, deployment := range allDeployments {
		if jobEnvironmentsMap[deployment.CreatedByJob] == nil {
			environments := make([]string, 1)
			environments[0] = deployment.Environment
			jobEnvironmentsMap[deployment.CreatedByJob] = environments
		} else {
			environments := jobEnvironmentsMap[deployment.CreatedByJob]
			environments = append(environments, deployment.Environment)
			jobEnvironmentsMap[deployment.CreatedByJob] = environments
		}
	}

	return jobEnvironmentsMap, nil
}

func (jh JobHandler) getPipelinePod(appName, jobName string) (*corev1.Pod, error) {
	ns := crdUtils.GetAppNamespace(appName)
	pods, err := jh.userAccount.Client.CoreV1().Pods(ns).List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", "job-name", jobName),
	})

	if err != nil {
		return nil, err
	}
	if len(pods.Items) == 0 {
		// pipeline pod not found
		return nil, nil
	}

	return &pods.Items[0], nil
}

func (jh JobHandler) getAllJobsLegacy() (*batchv1.JobList, error) {
	return jh.getJobsInNamespaceLegacy(jh.serviceAccount.Client, corev1.NamespaceAll)
}

func (jh JobHandler) getJobsLegacy(appName string) (*batchv1.JobList, error) {
	return jh.getJobsInNamespaceLegacy(jh.userAccount.Client, crdUtils.GetAppNamespace(appName))
}

func (jh JobHandler) getJobsInNamespaceLegacy(kubeClient kubernetes.Interface, namespace string) (*batchv1.JobList, error) {
	jobList, err := kubeClient.BatchV1().Jobs(namespace).List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", kube.RadixJobTypeLabel, RadixJobTypeJob),
	})

	if err != nil {
		return nil, err
	}

	return jobList, nil
}

func getPipelineJobStep(pipelinePod *corev1.Pod) jobModels.Step {
	var pipelineJobStep jobModels.Step

	cloneContainerStatus := getCloneContainerStatus(pipelinePod)
	if cloneContainerStatus == nil {
		return jobModels.Step{}
	}

	if cloneContainerStatus.State.Terminated != nil &&
		cloneContainerStatus.State.Terminated.ExitCode > 0 {
		pipelineJobStep = getJobStepWithContainerName(pipelinePod.GetName(),
			pipelinePod.Status.ContainerStatuses[0].Name, cloneContainerStatus)
	} else {
		pipelineJobStep = getJobStep(pipelinePod.GetName(), &pipelinePod.Status.ContainerStatuses[0])
	}

	return pipelineJobStep
}

func getCloneContainerStatus(pipelinePod *corev1.Pod) *corev1.ContainerStatus {
	for _, containerStatus := range pipelinePod.Status.InitContainerStatuses {
		if containerStatus.Name == git.CloneContainerName {
			return &containerStatus
		}
	}

	return nil
}

func getJobStep(podName string, containerStatus *corev1.ContainerStatus) jobModels.Step {
	return getJobStepWithContainerName(podName, containerStatus.Name, containerStatus)
}

func getJobStepWithContainerName(podName, containerName string, containerStatus *corev1.ContainerStatus) jobModels.Step {
	var startedAt metav1.Time
	var finishedAt metav1.Time

	status := jobModels.Succeeded

	if containerStatus == nil {
		status = jobModels.Waiting

	} else if containerStatus.State.Terminated != nil {
		startedAt = containerStatus.State.Terminated.StartedAt
		finishedAt = containerStatus.State.Terminated.FinishedAt

		if containerStatus.State.Terminated.ExitCode > 0 {
			status = jobModels.Failed
		}

	} else if containerStatus.State.Running != nil {
		startedAt = containerStatus.State.Running.StartedAt
		status = jobModels.Running

	} else if containerStatus.State.Waiting != nil {
		status = jobModels.Waiting

	}

	return jobModels.Step{
		Name:    containerName,
		Started: utils.FormatTime(&startedAt),
		Ended:   utils.FormatTime(&finishedAt),
		Status:  status.String(),
		PodName: podName,
	}
}
