package jobs

import (
	"sort"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"fmt"
	"strings"

	deployments "github.com/statoil/radix-api/api/deployments"
	deploymentModels "github.com/statoil/radix-api/api/deployments/models"
	jobModels "github.com/statoil/radix-api/api/jobs/models"
	"github.com/statoil/radix-api/api/utils"
	crdUtils "github.com/statoil/radix-operator/pkg/apis/utils"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
)

const workerImage = "radix-pipeline"
const dockerRegistry = "radixdev.azurecr.io"

// JobHandler Instance variables
type JobHandler struct {
	client      kubernetes.Interface
	radixclient radixclient.Interface
}

// Init Constructor
func Init(client kubernetes.Interface, radixclient radixclient.Interface) JobHandler {
	return JobHandler{client, radixclient}
}

// HandleGetApplicationJobs Handler for GetApplicationJobs
func (jh JobHandler) HandleGetApplicationJobs(appName string) ([]*jobModels.JobSummary, error) {
	return jh.getApplicationJobs(appName)
}

// HandleGetLatestApplicationJob Get last run application job
func (jh JobHandler) HandleGetLatestApplicationJob(appName string) (*jobModels.JobSummary, error) {
	jobs, err := jh.getApplicationJobs(appName)
	if err != nil {
		return nil, err
	}

	if len(jobs) == 0 {
		return nil, nil
	}

	return jobs[0], nil
}

func (jh JobHandler) getApplicationJobs(appName string) ([]*jobModels.JobSummary, error) {
	jobList, err := getJobs(jh.client, appName)
	if err != nil {
		return nil, err
	}

	// Sort jobs descending
	sort.Slice(jobList.Items, func(i, j int) bool {
		return jobList.Items[j].Status.StartTime.Before(jobList.Items[i].Status.StartTime)
	})

	jobs := make([]*jobModels.JobSummary, len(jobList.Items))
	deploy := deployments.Init(jh.client, jh.radixclient)
	for i, job := range jobList.Items {
		jobSummary := GetJobSummary(&job)
		jobDeployments, err := deploy.GetDeploymentsForJob(jh.radixclient, appName, jobSummary.Name)
		if err != nil {
			return nil, err
		}

		environments := make([]string, len(jobDeployments))
		for num, jobDeployment := range jobDeployments {
			environments[num] = jobDeployment.Environment
		}

		jobSummary.Environments = environments
		jobs[i] = jobSummary
	}

	return jobs, nil
}

// HandleGetApplicationJob Handler for GetApplicationJob
func (jh JobHandler) HandleGetApplicationJob(appName, jobName string) (*jobModels.Job, error) {
	job, err := jh.client.BatchV1().Jobs(crdUtils.GetAppNamespace(appName)).Get(jobName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if !strings.EqualFold(job.Labels["radix-job-type"], "job") {
		return nil, utils.ValidationError("Radix Application Job", "Job was not of expected type")
	}

	steps, err := jh.getJobSteps(appName, job)
	if err != nil {
		return nil, err
	}
	deploy := deployments.Init(jh.client, jh.radixclient)
	jobDeployments, err := deploy.GetDeploymentsForJob(jh.radixclient, appName, jobName)
	if err != nil {
		return nil, err
	}
	components := []*deploymentModels.ComponentSummary{}
	if len(jobDeployments) > 0 {
		components = jobDeployments[0].Components
	}

	jobStatus := jobModels.GetStatusFromJobStatus(job.Status)
	var jobEnded metav1.Time

	if len(job.Status.Conditions) > 0 {
		jobEnded = job.Status.Conditions[0].LastTransitionTime
	}

	return &jobModels.Job{
		Name:        job.Name,
		Branch:      job.Labels["radix-branch"],
		CommitID:    job.Labels["radix-commit"],
		Started:     utils.FormatTime(job.Status.StartTime),
		Ended:       utils.FormatTime(&jobEnded),
		Status:      jobStatus.String(),
		Pipeline:    job.Labels["radix-pipeline"],
		Steps:       steps,
		Deployments: jobDeployments,
		Components:  components,
	}, nil
}

func (jh JobHandler) getJobSteps(appName string, job *batchv1.Job) ([]jobModels.Step, error) {
	labelSelector := fmt.Sprintf("radix-image-tag=%s, radix-job-type!=%s", job.Labels["radix-image-tag"], "job")
	jobStepList, err := jh.client.BatchV1().Jobs(crdUtils.GetAppNamespace(appName)).List(metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	steps := make([]jobModels.Step, 0)
	for _, jobStep := range jobStepList.Items {
		jobStepPod, err := jh.client.CoreV1().Pods(crdUtils.GetAppNamespace(appName)).List(metav1.ListOptions{
			LabelSelector: fmt.Sprintf("job-name=%s", jobStep.Name),
		})

		if err != nil {
			return nil, err
		}

		if len(jobStepPod.Items) == 0 {
			continue
		}

		pod := jobStepPod.Items[0]
		for _, containerStatus := range pod.Status.InitContainerStatuses {
			steps = append(steps, getJobStep(&containerStatus))
		}

		for _, containerStatus := range pod.Status.ContainerStatuses {
			steps = append(steps, getJobStep(&containerStatus))
		}
	}
	return steps, nil
}

// GetJobSummary Used to get job summary from a kubernetes job
func GetJobSummary(job *batchv1.Job) *jobModels.JobSummary {
	appName := job.Labels["radix-app-name"]
	branch := job.Labels["radix-branch"]
	commit := job.Labels["radix-commit"]
	pipeline := job.Labels["radix-pipeline"]

	status := job.Status

	jobStatus := jobModels.GetStatusFromJobStatus(status)
	ended := utils.FormatTime(status.CompletionTime)
	if jobStatus == jobModels.Failed {
		ended = utils.FormatTime(&status.Conditions[0].LastTransitionTime)
	}

	pipelineJob := &jobModels.JobSummary{
		Name:     job.Name,
		AppName:  appName,
		Branch:   branch,
		CommitID: commit,
		Status:   jobStatus.String(),
		Started:  utils.FormatTime(status.StartTime),
		Ended:    ended,
		Pipeline: pipeline,
	}
	return pipelineJob
}

func getJobs(client kubernetes.Interface, appName string) (*batchv1.JobList, error) {
	jobList, err := client.BatchV1().Jobs(crdUtils.GetAppNamespace(appName)).List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("radix-job-type=%s", "job"),
	})

	if err != nil {
		return nil, err
	}

	return jobList, nil
}

func getJobStep(containerStatus *corev1.ContainerStatus) jobModels.Step {
	var startedAt metav1.Time
	var finishedAt metav1.Time

	status := jobModels.Succeeded

	if containerStatus.State.Terminated != nil {
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
		Name:    containerStatus.Name,
		Started: utils.FormatTime(&startedAt),
		Ended:   utils.FormatTime(&finishedAt),
		Status:  status.String(),
	}
}
