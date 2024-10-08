package jobs

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/equinor/radix-api/api/deployments"
	jobModels "github.com/equinor/radix-api/api/jobs/models"
	"github.com/equinor/radix-api/api/kubequery"
	"github.com/equinor/radix-api/api/utils"
	"github.com/equinor/radix-api/api/utils/tekton"
	"github.com/equinor/radix-api/models"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

const (
	WorkerImage              = "radix-pipeline"
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

// GetApplicationJobs Handler for GetApplicationJobs
func (jh JobHandler) GetApplicationJobs(ctx context.Context, appName string) ([]*jobModels.JobSummary, error) {
	jobs, err := jh.getJobs(ctx, appName)
	if err != nil {
		return nil, err
	}

	// Sort jobs descending
	sort.Slice(jobs, func(i, j int) bool {
		return utils.IsBefore(jobs[j], jobs[i])
	})

	return jobs, nil
}

// GetApplicationJob Handler for GetApplicationJob
func (jh JobHandler) GetApplicationJob(ctx context.Context, appName, jobName string) (*jobModels.Job, error) {
	job, err := jh.userAccount.RadixClient.RadixV1().RadixJobs(crdUtils.GetAppNamespace(appName)).Get(ctx, jobName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	jobDeployments, err := jh.deploy.GetDeploymentsForPipelineJob(ctx, appName, jobName)
	if err != nil {
		return nil, err
	}

	return jobModels.GetJobFromRadixJob(job, jobDeployments), nil
}

// GetTektonPipelineRuns Get the Tekton pipeline runs
func (jh JobHandler) GetTektonPipelineRuns(ctx context.Context, appName, jobName string) ([]jobModels.PipelineRun, error) {
	pipelineRuns, err := tekton.GetTektonPipelineRuns(ctx, jh.userAccount.TektonClient, appName, jobName)
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
func (jh JobHandler) GetTektonPipelineRun(ctx context.Context, appName, jobName, pipelineRunName string) (*jobModels.PipelineRun, error) {
	pipelineRun, err := tekton.GetPipelineRun(ctx, jh.userAccount.TektonClient, appName, jobName, pipelineRunName)
	if err != nil {
		return nil, err
	}
	return getPipelineRunModel(pipelineRun), nil
}

// GetTektonPipelineRunTasks Get the Tekton pipeline run tasks
func (jh JobHandler) GetTektonPipelineRunTasks(ctx context.Context, appName, jobName, pipelineRunName string) ([]jobModels.PipelineRunTask, error) {
	pipelineRun, taskNameToTaskRunMap, err := jh.getPipelineRunWithTasks(ctx, appName, jobName, pipelineRunName)
	if err != nil {
		return nil, err
	}
	taskModels := getPipelineRunTaskModels(pipelineRun, taskNameToTaskRunMap)
	return sortPipelineTasks(taskModels), nil
}

func (jh JobHandler) getPipelineRunWithTasks(ctx context.Context, appName string, jobName string, pipelineRunName string) (*pipelinev1.PipelineRun, map[string]*pipelinev1.TaskRun, error) {
	pipelineRun, err := jh.getPipelineRunWithRef(ctx, appName, jobName, pipelineRunName)
	if err != nil {
		return nil, nil, err
	}
	taskRunsMap, err := tekton.GetTektonPipelineTaskRuns(ctx, jh.userAccount.TektonClient, appName, jobName, pipelineRunName)
	if err != nil {
		return nil, nil, err
	}
	taskNameToTaskRunMap := getPipelineTaskNameToTaskRunMap(pipelineRun.Status.PipelineSpec.Tasks, taskRunsMap)
	return pipelineRun, taskNameToTaskRunMap, nil
}

func (jh JobHandler) getPipelineRunWithRef(ctx context.Context, appName string, jobName string, pipelineRunName string) (*pipelinev1.PipelineRun, error) {
	pipelineRun, err := tekton.GetPipelineRun(ctx, jh.userAccount.TektonClient, appName, jobName, pipelineRunName)
	if err != nil {
		return nil, err
	}
	if pipelineRun.Spec.PipelineRef == nil || len(pipelineRun.Spec.PipelineRef.Name) == 0 {
		return nil, fmt.Errorf("the Pipeline Run %s does not have reference to the Pipeline", pipelineRunName)
	}
	return pipelineRun, nil
}

func getPipelineTaskNameToTaskRunMap(pipelineTasks []pipelinev1.PipelineTask, taskRunsMap map[string]*pipelinev1.TaskRun) map[string]*pipelinev1.TaskRun {
	return slice.Reduce(pipelineTasks, make(map[string]*pipelinev1.TaskRun), func(acc map[string]*pipelinev1.TaskRun, task pipelinev1.PipelineTask) map[string]*pipelinev1.TaskRun {
		if task.TaskRef == nil {
			return acc
		}
		if taskRun, ok := taskRunsMap[task.TaskRef.Name]; ok {
			acc[task.Name] = taskRun
		}
		return acc
	})
}

// GetTektonPipelineRunTask Get the Tekton pipeline run task
func (jh JobHandler) GetTektonPipelineRunTask(ctx context.Context, appName, jobName, pipelineRunName, taskName string) (*jobModels.PipelineRunTask, error) {
	pipelineRun, taskRun, err := jh.getPipelineRunAndTaskRun(ctx, appName, jobName, pipelineRunName, taskName)
	if err != nil {
		return nil, err
	}
	return getPipelineRunTaskModelByTaskSpec(pipelineRun, taskRun), nil
}

// GetTektonPipelineRunTaskSteps Get the Tekton pipeline run task steps
func (jh JobHandler) GetTektonPipelineRunTaskSteps(ctx context.Context, appName, jobName, pipelineRunName, taskName string) ([]jobModels.PipelineRunTaskStep, error) {
	_, taskRun, err := jh.getPipelineRunAndTaskRun(ctx, appName, jobName, pipelineRunName, taskName)
	if err != nil {
		return nil, err
	}
	return buildPipelineRunTaskStepModels(taskRun), nil
}

func (jh JobHandler) getPipelineRunAndTaskRun(ctx context.Context, appName string, jobName string, pipelineRunName string, taskName string) (*pipelinev1.PipelineRun, *pipelinev1.TaskRun, error) {
	pipelineRun, err := jh.getPipelineRunWithRef(ctx, appName, jobName, pipelineRunName)
	if err != nil {
		return nil, nil, err
	}
	taskRun, err := tekton.GetTektonPipelineTaskRunByTaskName(ctx, jh.userAccount.TektonClient, appName, jobName, pipelineRunName, taskName)
	if err != nil {
		return nil, nil, err
	}
	return pipelineRun, taskRun, nil
}

func getPipelineRunModel(pipelineRun *pipelinev1.PipelineRun) *jobModels.PipelineRun {
	pipelineRunModel := jobModels.PipelineRun{
		Name:     pipelineRun.ObjectMeta.Annotations[tektonRealNameAnnotation],
		Env:      pipelineRun.ObjectMeta.Labels[kube.RadixEnvLabel],
		RealName: pipelineRun.GetName(),
		Started:  radixutils.FormatTime(pipelineRun.Status.StartTime),
		Ended:    radixutils.FormatTime(pipelineRun.Status.CompletionTime),
	}
	runCondition := getLastReadyCondition(pipelineRun.Status.Conditions)
	if runCondition != nil {
		pipelineRunModel.Status = jobModels.TaskRunReason(runCondition.Reason)
		pipelineRunModel.StatusMessage = runCondition.Message
	}
	return &pipelineRunModel
}

func getPipelineRunTaskModels(pipelineRun *pipelinev1.PipelineRun, taskNameToTaskRunMap map[string]*pipelinev1.TaskRun) []jobModels.PipelineRunTask {
	return slice.Reduce(pipelineRun.Status.ChildReferences, make([]jobModels.PipelineRunTask, 0), func(acc []jobModels.PipelineRunTask, taskRunSpec pipelinev1.ChildStatusReference) []jobModels.PipelineRunTask {
		if taskRun, ok := taskNameToTaskRunMap[taskRunSpec.PipelineTaskName]; ok {
			pipelineTaskModel := getPipelineRunTaskModelByTaskSpec(pipelineRun, taskRun)
			acc = append(acc, *pipelineTaskModel)
		}
		return acc
	})
}

func getPipelineRunTaskModelByTaskSpec(pipelineRun *pipelinev1.PipelineRun, taskRun *pipelinev1.TaskRun) *jobModels.PipelineRunTask {
	pipelineTaskModel := jobModels.PipelineRunTask{
		Name:           taskRun.ObjectMeta.Labels["tekton.dev/pipelineTask"],
		RealName:       taskRun.Spec.TaskRef.Name,
		PipelineRunEnv: pipelineRun.ObjectMeta.Labels[kube.RadixEnvLabel],
		PipelineName:   pipelineRun.ObjectMeta.Annotations[tektonRealNameAnnotation],
	}
	pipelineTaskModel.Started = radixutils.FormatTime(taskRun.Status.StartTime)
	pipelineTaskModel.Ended = radixutils.FormatTime(taskRun.Status.CompletionTime)
	taskCondition := getLastReadyCondition(taskRun.Status.Conditions)
	if taskCondition != nil {
		pipelineTaskModel.Status = jobModels.PipelineRunReason(taskCondition.Reason)
		pipelineTaskModel.StatusMessage = taskCondition.Message
	}
	logEmbeddedCommandIndex := strings.Index(pipelineTaskModel.StatusMessage, "for logs run")
	if logEmbeddedCommandIndex >= 0 { // Avoid to publish kubectl command, provided by Tekton component after "for logs run" prefix for failed task step
		pipelineTaskModel.StatusMessage = pipelineTaskModel.StatusMessage[0:logEmbeddedCommandIndex]
	}
	return &pipelineTaskModel
}

func buildPipelineRunTaskStepModels(taskRun *pipelinev1.TaskRun) []jobModels.PipelineRunTaskStep {
	var stepsModels []jobModels.PipelineRunTaskStep
	for _, stepStatus := range taskRun.Status.TaskRunStatusFields.Steps {
		stepModel := jobModels.PipelineRunTaskStep{Name: stepStatus.Name}
		if stepStatus.Terminated != nil {
			stepModel.Started = radixutils.FormatTime(&stepStatus.Terminated.StartedAt)
			stepModel.Ended = radixutils.FormatTime(&stepStatus.Terminated.FinishedAt)
			stepModel.Status = jobModels.TaskRunReason(stepStatus.Terminated.Reason)
			stepModel.StatusMessage = stepStatus.Terminated.Message
		} else if stepStatus.Running != nil {
			stepModel.Started = radixutils.FormatTime(&stepStatus.Running.StartedAt)
			stepModel.Status = jobModels.TaskRunReasonRunning
		} else if stepStatus.Waiting != nil {
			stepModel.Status = jobModels.TaskRunReason(stepStatus.Waiting.Reason)
			stepModel.StatusMessage = stepStatus.Waiting.Message
		}
		stepsModels = append(stepsModels, stepModel)
	}
	return stepsModels
}

func getLastReadyCondition(conditions []apis.Condition) *apis.Condition {
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

func sortPipelineTaskStatusConditionsDesc(conditions []apis.Condition) duckv1.Conditions {
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

func (jh JobHandler) getJobs(ctx context.Context, appName string) ([]*jobModels.JobSummary, error) {
	jobs, err := kubequery.GetRadixJobs(ctx, jh.accounts.UserAccount.RadixClient, appName)
	if err != nil {
		return nil, err
	}

	return slice.Map(jobs, func(j v1.RadixJob) *jobModels.JobSummary {
		return jobModels.GetSummaryFromRadixJob(&j)
	}), nil
}
