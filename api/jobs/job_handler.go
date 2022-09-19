package jobs

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/equinor/radix-api/api/deployments"
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	jobModels "github.com/equinor/radix-api/api/jobs/models"
	"github.com/equinor/radix-api/api/utils"
	"github.com/equinor/radix-api/api/utils/tekton"
	"github.com/equinor/radix-api/models"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/go-openapi/errors"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	knative "knative.dev/pkg/apis/duck/v1beta1"
)

const (
	workerImage              = "radix-pipeline"
	tektonRealNameAnnotation = "radix.equinor.com/tekton-pipeline-name"
)

// JobHandler Instance variables
type JobHandler struct {
	accounts       models.Accounts
	userAccount    models.Account
	serviceAccount models.Account
	deploy         deployments.DeployHandler
}

// Init Constructor
func Init(accounts models.Accounts, deployHandler deployments.DeployHandler) JobHandler {
	return JobHandler{
		accounts:       accounts,
		userAccount:    accounts.UserAccount,
		serviceAccount: accounts.ServiceAccount,
		deploy:         deployHandler,
	}
}

// GetLatestJobPerApplication Handler for GetApplicationJobs - NOTE: does not get latestJob.Environments
func (jh JobHandler) GetLatestJobPerApplication(forApplications map[string]bool) (map[string]*jobModels.JobSummary, error) {
	return jh.getLatestJobPerApplication(forApplications)
}

// GetApplicationJobs Handler for GetApplicationJobs
func (jh JobHandler) GetApplicationJobs(appName string) ([]*jobModels.JobSummary, error) {
	return jh.getApplicationJobs(appName)
}

// GetLatestApplicationJob Get last run application job
func (jh JobHandler) GetLatestApplicationJob(appName string) (*jobModels.JobSummary, error) {
	jobs, err := jh.getApplicationJobs(appName)
	if err != nil {
		return nil, err
	}

	if len(jobs) == 0 {
		return nil, nil
	}

	return jobs[0], nil
}

// GetApplicationJob Handler for GetApplicationJob
func (jh JobHandler) GetApplicationJob(appName, jobName string) (*jobModels.Job, error) {
	job, err := jh.userAccount.RadixClient.RadixV1().RadixJobs(crdUtils.GetAppNamespace(appName)).Get(context.TODO(), jobName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	jobDeployments, err := jh.deploy.GetDeploymentsForJob(appName, jobName)
	if err != nil {
		return nil, err
	}

	jobComponents, err := jh.getJobComponents(appName, jobDeployments)
	if err != nil {
		return nil, err
	}

	return jobModels.GetJobFromRadixJob(job, jobDeployments, jobComponents), nil
}

// GetTektonPipelineRuns Get the Tekton pipeline runs
func (jh JobHandler) GetTektonPipelineRuns(appName, jobName string) ([]jobModels.PipelineRun, error) {
	pipelineRuns, err := tekton.GetTektonPipelineRuns(jh.userAccount.TektonClient, appName, jobName)
	if err != nil {
		return nil, err
	}
	var pipelineRunModels []jobModels.PipelineRun
	for _, pipelineRun := range pipelineRuns {
		pipelineRunModel := getPipelineRunModel(&pipelineRun)
		pipelineRunModels = append(pipelineRunModels, *pipelineRunModel)
	}
	return pipelineRunModels, nil
}

// GetTektonPipelineRun Get the Tekton pipeline run
func (jh JobHandler) GetTektonPipelineRun(appName, jobName, pipelineRunName string) (*jobModels.PipelineRun, error) {
	pipelineRun, err := tekton.GetPipelineRun(jh.userAccount.TektonClient, appName, jobName, pipelineRunName)
	if err != nil {
		return nil, err
	}
	return getPipelineRunModel(pipelineRun), nil
}

// GetTektonPipelineRunTasks Get the Tekton pipeline run tasks
func (jh JobHandler) GetTektonPipelineRunTasks(appName, jobName, pipelineRunName string) ([]jobModels.PipelineRunTask, error) {
	pipelineRun, taskNameToRealNameMap, err := jh.getPipelineRunWithTasks(appName, jobName, pipelineRunName)
	if err != nil {
		return nil, err
	}
	taskModels := getPipelineRunTaskModels(pipelineRun, taskNameToRealNameMap)
	return sortPipelineTasks(taskModels), nil
}

func (jh JobHandler) getPipelineRunWithTasks(appName string, jobName string, pipelineRunName string) (*v1beta1.PipelineRun, map[string]string, error) {
	pipelineRun, err := tekton.GetPipelineRun(jh.userAccount.TektonClient, appName, jobName, pipelineRunName)
	if err != nil {
		return nil, nil, err
	}
	if pipelineRun.Spec.PipelineRef == nil || len(pipelineRun.Spec.PipelineRef.Name) == 0 {
		return nil, nil, fmt.Errorf("the Pipeline Run %s does not have reference to the Pipeline", pipelineRunName)
	}
	taskNameToRealNameMap := make(map[string]string, len(pipelineRun.Status.PipelineSpec.Tasks))
	for _, task := range pipelineRun.Status.PipelineSpec.Tasks {
		if task.TaskRef != nil {
			taskNameToRealNameMap[task.Name] = task.TaskRef.Name
		}
	}
	return pipelineRun, taskNameToRealNameMap, nil
}

// GetTektonPipelineRunTask Get the Tekton pipeline run task
func (jh JobHandler) GetTektonPipelineRunTask(appName, jobName, pipelineRunName, taskName string) (*jobModels.PipelineRunTask, error) {
	pipelineRun, taskRunSpec, taskRealName, err := jh.getPipelineRunAndTask(appName, jobName, pipelineRunName, taskName)
	if err != nil {
		return nil, err
	}
	return getPipelineRunTaskModelByTaskSpec(pipelineRun, taskRunSpec, taskRealName), nil
}

// GetTektonPipelineRunTaskSteps Get the Tekton pipeline run task steps
func (jh JobHandler) GetTektonPipelineRunTaskSteps(appName, jobName, pipelineRunName, taskName string) ([]jobModels.PipelineRunTaskStep, error) {
	_, taskRunSpec, _, err := jh.getPipelineRunAndTask(appName, jobName, pipelineRunName, taskName)
	if err != nil {
		return nil, err
	}
	return buildPipelineRunTaskStepModels(taskRunSpec), nil
}

func (jh JobHandler) getPipelineRunAndTask(appName string, jobName string, pipelineRunName string, taskName string) (*v1beta1.PipelineRun, *v1beta1.PipelineRunTaskRunStatus, string, error) {
	pipelineRun, taskNameToRealNameMap, err := jh.getPipelineRunWithTasks(appName, jobName, pipelineRunName)
	if err != nil {
		return nil, nil, "", err
	}
	taskRunSpec, taskRealName, err := getPipelineRunTaskSpecByName(pipelineRun, taskNameToRealNameMap, taskName)
	if err != nil {
		return nil, nil, "", err
	}
	return pipelineRun, taskRunSpec, taskRealName, nil
}

func getPipelineRunModel(pipelineRun *v1beta1.PipelineRun) *jobModels.PipelineRun {
	pipelineRunModel := jobModels.PipelineRun{
		Name:     pipelineRun.ObjectMeta.Annotations[tektonRealNameAnnotation],
		Env:      pipelineRun.ObjectMeta.Labels[kube.RadixEnvLabel],
		RealName: pipelineRun.GetName(),
		Started:  radixutils.FormatTime(pipelineRun.Status.StartTime),
		Ended:    radixutils.FormatTime(pipelineRun.Status.CompletionTime),
	}
	runCondition := getLastReadyCondition(pipelineRun.Status.Conditions)
	if runCondition != nil {
		pipelineRunModel.Status = runCondition.Reason
		pipelineRunModel.StatusMessage = runCondition.Message
	}
	return &pipelineRunModel
}

func getPipelineRunTaskModels(pipelineRun *v1beta1.PipelineRun, taskNameToRealNameMap map[string]string) []jobModels.PipelineRunTask {
	var taskModels []jobModels.PipelineRunTask
	for _, taskRunSpec := range pipelineRun.Status.TaskRuns {
		if taskRealName, ok := taskNameToRealNameMap[taskRunSpec.PipelineTaskName]; ok {
			pipelineTaskModel := getPipelineRunTaskModelByTaskSpec(pipelineRun, taskRunSpec, taskRealName)
			taskModels = append(taskModels, *pipelineTaskModel)
		}
	}
	return taskModels
}

func getPipelineRunTaskModelByTaskSpec(pipelineRun *v1beta1.PipelineRun, taskRunSpec *v1beta1.PipelineRunTaskRunStatus, realTaskName string) *jobModels.PipelineRunTask {
	pipelineTaskModel := jobModels.PipelineRunTask{
		Name:           taskRunSpec.PipelineTaskName,
		RealName:       realTaskName,
		PipelineRunEnv: pipelineRun.ObjectMeta.Labels[kube.RadixEnvLabel],
		PipelineName:   pipelineRun.ObjectMeta.Annotations[tektonRealNameAnnotation],
	}
	if taskRunSpec.Status != nil {
		pipelineTaskModel.Started = radixutils.FormatTime(taskRunSpec.Status.StartTime)
		pipelineTaskModel.Ended = radixutils.FormatTime(taskRunSpec.Status.CompletionTime)
		taskCondition := getLastReadyCondition(taskRunSpec.Status.Conditions)
		if taskCondition != nil {
			pipelineTaskModel.Status = taskCondition.Reason
			pipelineTaskModel.StatusMessage = taskCondition.Message
		}
	}
	logEmbeddedCommandIndex := strings.Index(pipelineTaskModel.StatusMessage, "for logs run")
	if logEmbeddedCommandIndex >= 0 { //Avoid to publish kubectl command, provided by Tekton component after "for logs run" prefix for failed task step
		pipelineTaskModel.StatusMessage = pipelineTaskModel.StatusMessage[0:logEmbeddedCommandIndex]
	}
	return &pipelineTaskModel
}

func getPipelineRunTaskSpecByName(pipelineRun *v1beta1.PipelineRun, taskNameToRealNameMap map[string]string, taskName string) (*v1beta1.PipelineRunTaskRunStatus, string, error) {
	for _, taskRunSpec := range pipelineRun.Status.TaskRuns {
		if taskRealName, ok := taskNameToRealNameMap[taskRunSpec.PipelineTaskName]; ok && strings.EqualFold(taskRealName, taskName) {
			return taskRunSpec, taskRealName, nil
		}
	}
	return nil, "", errors.NotFound("task %s not found", taskName)
}

func buildPipelineRunTaskStepModels(taskRunSpec *v1beta1.PipelineRunTaskRunStatus) []jobModels.PipelineRunTaskStep {
	var stepsModels []jobModels.PipelineRunTaskStep
	for _, stepStatus := range taskRunSpec.Status.TaskRunStatusFields.Steps {
		stepModel := jobModels.PipelineRunTaskStep{Name: stepStatus.Name}
		if stepStatus.Terminated != nil {
			stepModel.Started = radixutils.FormatTime(&stepStatus.Terminated.StartedAt)
			stepModel.Ended = radixutils.FormatTime(&stepStatus.Terminated.FinishedAt)
			stepModel.Status = stepStatus.Terminated.Reason
			stepModel.StatusMessage = stepStatus.Terminated.Message
		} else if stepStatus.Running != nil {
			stepModel.Started = radixutils.FormatTime(&stepStatus.Running.StartedAt)
			stepModel.Status = jobModels.Running.String()
		} else if stepStatus.Waiting != nil {
			stepModel.Status = stepStatus.Waiting.Reason
			stepModel.StatusMessage = stepStatus.Waiting.Message
		}
		stepsModels = append(stepsModels, stepModel)
	}
	return stepsModels
}

//lint:ignore U1000 decide if we should use it for sorting task results
func sortPipelineTaskSteps(steps []jobModels.PipelineRunTaskStep) []jobModels.PipelineRunTaskStep {
	sort.Slice(steps, func(i, j int) bool {
		if steps[i].Started == "" || steps[j].Started == "" {
			return false
		}
		return steps[i].Started < steps[j].Started
	})
	return steps
}

func getLastReadyCondition(conditions knative.Conditions) *apis.Condition {
	if len(conditions) == 1 {
		return &conditions[0]
	}
	conditions = sortPipelineTaskStatusConditionsDesc(conditions)
	for _, condition := range conditions {
		if condition.Status == corev1.ConditionTrue {
			return &condition
		}
	}
	if len(conditions) > 0 {
		return &conditions[0]
	}
	return nil
}

func sortPipelineTaskStatusConditionsDesc(conditions knative.Conditions) knative.Conditions {
	sort.Slice(conditions, func(i, j int) bool {
		if conditions[i].LastTransitionTime.Inner.IsZero() || conditions[j].LastTransitionTime.Inner.IsZero() {
			return false
		}
		return conditions[j].LastTransitionTime.Inner.Before(&conditions[i].LastTransitionTime.Inner)
	})
	return conditions
}

func sortPipelineTasks(tasks []jobModels.PipelineRunTask) []jobModels.PipelineRunTask {
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].Started == "" || tasks[j].Started == "" {
			return false
		}
		return tasks[i].Started < tasks[j].Started
	})
	return tasks
}

func (jh JobHandler) getApplicationJobs(appName string) ([]*jobModels.JobSummary, error) {
	jobs, err := jh.getJobs(appName)
	if err != nil {
		return nil, err
	}

	// Sort jobs descending
	sort.Slice(jobs, func(i, j int) bool {
		return utils.IsBefore(jobs[j], jobs[i])
	})

	return jobs, nil
}

func (jh JobHandler) getDefinedJobs(appNames []string) ([]*jobModels.JobSummary, error) {
	var g errgroup.Group
	g.SetLimit(25)

	jobsCh := make(chan []*jobModels.JobSummary, len(appNames))
	for _, appName := range appNames {
		name := appName // locally scope appName to avoid race condition in go routines
		g.Go(func() error {
			jobs, err := jh.getJobs(name)
			if err == nil {
				jobsCh <- jobs
			}
			return err
		})
	}

	err := g.Wait()
	close(jobsCh)
	if err != nil {
		return nil, err
	}

	var jobSummaries []*jobModels.JobSummary
	for jobs := range jobsCh {
		jobSummaries = append(jobSummaries, jobs...)
	}
	return jobSummaries, nil
}

func (jh JobHandler) getJobs(appName string) ([]*jobModels.JobSummary, error) {
	return jh.getJobsInNamespace(crdUtils.GetAppNamespace(appName))
}

func (jh JobHandler) getJobsInNamespace(namespace string) ([]*jobModels.JobSummary, error) {
	jobList, err := jh.userAccount.RadixClient.RadixV1().RadixJobs(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	jobs := make([]*jobModels.JobSummary, len(jobList.Items))
	for i, job := range jobList.Items {
		jobs[i] = jobModels.GetSummaryFromRadixJob(&job)
	}

	return jobs, nil
}

func (jh JobHandler) getLatestJobPerApplication(forApplications map[string]bool) (map[string]*jobModels.JobSummary, error) {
	// Primarily use Radix Jobs
	var apps []string
	for name, shouldAdd := range forApplications {
		if shouldAdd {
			apps = append(apps, name)
		}
	}

	someJobs, err := jh.getDefinedJobs(apps)
	if err != nil {
		return nil, err
	}

	sort.Slice(someJobs, func(i, j int) bool {
		switch strings.Compare(someJobs[i].AppName, someJobs[j].AppName) {
		case -1:
			return true
		case 1:
			return false
		}

		return utils.IsBefore(someJobs[j], someJobs[i])
	})

	applicationJob := make(map[string]*jobModels.JobSummary)
	for _, job := range someJobs {
		if applicationJob[job.AppName] != nil {
			continue
		}
		if !forApplications[job.AppName] {
			continue
		}

		if job.Started == "" {
			// Job may still be queued or waiting to be scheduled by the operator
			continue
		}

		applicationJob[job.AppName] = job
	}

	forApplicationsWithNoRadixJob := make(map[string]bool)
	for applicationName := range forApplications {
		if applicationJob[applicationName] == nil {
			forApplicationsWithNoRadixJob[applicationName] = true
		}
	}

	return applicationJob, nil
}

func (jh JobHandler) getJobComponents(appName string, jobDeployments []*deploymentModels.DeploymentSummary) ([]*deploymentModels.ComponentSummary, error) {
	var jobComponents []*deploymentModels.ComponentSummary

	if len(jobDeployments) > 0 {
		// All deployments for a job should have the same components, so we extract the components from the first one

		firstDeployment, err := jh.deploy.GetDeploymentWithName(appName, jobDeployments[0].Name)
		if err != nil {
			return nil, err
		}

		for _, component := range firstDeployment.Components {
			componentSummary := deploymentModels.ComponentSummary{
				Name:  component.Name,
				Type:  component.Type,
				Image: component.Image,
			}
			jobComponents = append(jobComponents, &componentSummary)
		}
	}

	return jobComponents, nil
}
