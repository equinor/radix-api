package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"sort"
	"strings"

	"github.com/equinor/radix-api/api/deployments"
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	jobModels "github.com/equinor/radix-api/api/jobs/models"
	"github.com/equinor/radix-api/api/utils"
	"github.com/equinor/radix-api/models"
	radixhttp "github.com/equinor/radix-common/net/http"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	knative "knative.dev/pkg/apis/duck/v1beta1"
)

const workerImage = "radix-pipeline"

func stepNotFoundError(stepName string) error {
	return radixhttp.NotFoundError(fmt.Sprintf("step %s not found", stepName))
}

func stepScanOutputNotDefined(stepName string) error {
	return radixhttp.NotFoundError(fmt.Sprintf("scan output for step %s not defined", stepName))
}

func stepScanOutputInvalidConfig(stepName string) error {
	return &radixhttp.Error{
		Type:    radixhttp.Server,
		Message: fmt.Sprintf("scan output configuration for step %s is invalid", stepName),
	}
}

func stepScanOutputMissingKeyInConfigMap(stepName string) error {
	return &radixhttp.Error{
		Type:    radixhttp.Server,
		Message: fmt.Sprintf("scan output data for step %s not found", stepName),
	}
}

func stepScanOutputInvalidConfigMapData(stepName string) error {
	return &radixhttp.Error{
		Type:    radixhttp.Server,
		Message: fmt.Sprintf("scan output data for step %s is invalid", stepName),
	}
}

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

// GetPipelineJobStepScanOutput Get vulnerability scan output from scan step
func (jh JobHandler) GetPipelineJobStepScanOutput(appName, jobName, stepName string) ([]jobModels.Vulnerability, error) {
	namespace := crdUtils.GetAppNamespace(appName)
	job, err := jh.userAccount.RadixClient.RadixV1().RadixJobs(namespace).Get(context.TODO(), jobName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	step := getStepFromRadixJob(job, stepName)
	if step == nil {
		return nil, stepNotFoundError(stepName)
	}

	if step.Output == nil || step.Output.Scan == nil {
		return nil, stepScanOutputNotDefined(stepName)
	}

	scanOutputName, scanOutputKey := strings.TrimSpace(step.Output.Scan.VulnerabilityListConfigMap), strings.TrimSpace(step.Output.Scan.VulnerabilityListKey)
	if scanOutputName == "" || scanOutputKey == "" {
		return nil, stepScanOutputInvalidConfig(stepName)
	}

	scanOutputConfigMap, err := jh.userAccount.Client.CoreV1().ConfigMaps(namespace).Get(context.TODO(), scanOutputName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	scanOutput, found := scanOutputConfigMap.Data[scanOutputKey]
	if !found {
		return nil, stepScanOutputMissingKeyInConfigMap(stepName)
	}

	var vulnerabilities []jobModels.Vulnerability
	if err := json.Unmarshal([]byte(scanOutput), &vulnerabilities); err != nil {
		return nil, stepScanOutputInvalidConfigMapData(stepName)
	}

	return vulnerabilities, nil
}

// GetTektonPipelineRuns Get the Tekton pipeline-runs
func (jh JobHandler) GetTektonPipelineRuns(appName, jobName string) ([]jobModels.PipelineRun, error) {
	namespace := crdUtils.GetAppNamespace(appName)
	pipelineRunList, err := jh.userAccount.TektonClient.TektonV1beta1().PipelineRuns(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", kube.RadixJobNameLabel, jobName),
	})
	if err != nil {
		return nil, err
	}
	var pipelineRuns []jobModels.PipelineRun
	for _, pipelineRun := range pipelineRunList.Items {
		pipelineRunModel := jobModels.PipelineRun{
			Name:     pipelineRun.ObjectMeta.Annotations["radix.equinor.com/tekton-pipeline-name"],
			RealName: pipelineRun.GetName(),
			Started:  radixutils.FormatTime(pipelineRun.Status.StartTime),
			Ended:    radixutils.FormatTime(pipelineRun.Status.CompletionTime),
		}
		pipelineRunModel.Tasks = getPipelineRunTaskModels(&pipelineRun)
		runCondition := getLastReadyCondition(pipelineRun.Status.Conditions)
		if runCondition != nil {
			pipelineRunModel.Status = runCondition.Reason
			pipelineRunModel.StatusMessage = runCondition.Message
		}
		pipelineRuns = append(pipelineRuns, pipelineRunModel)
	}
	return pipelineRuns, nil
}

func getPipelineRunTaskModels(pipelineRun *v1beta1.PipelineRun) []jobModels.PipelineTask {
	var taskModels []jobModels.PipelineTask
	for realTaskName, taskRunSpec := range pipelineRun.Status.TaskRuns {
		pipelineTaskModel := jobModels.PipelineTask{
			Name:     taskRunSpec.PipelineTaskName,
			RealName: realTaskName,
		}
		if taskRunSpec.Status != nil {
			pipelineTaskModel.Started = radixutils.FormatTime(taskRunSpec.Status.StartTime)
			pipelineTaskModel.Ended = radixutils.FormatTime(taskRunSpec.Status.CompletionTime)
			pipelineTaskModel.PodName = taskRunSpec.Status.PodName
			taskCondition := getLastReadyCondition(taskRunSpec.Status.Conditions)
			if taskCondition != nil {
				pipelineTaskModel.Status = taskCondition.Reason
				pipelineTaskModel.StatusMessage = taskCondition.Message
			}
		}
		taskModels = append(taskModels, pipelineTaskModel)
	}
	return sortPipelineTasks(taskModels)
}

func getLastReadyCondition(conditions knative.Conditions) *apis.Condition {
	var taskConditions knative.Conditions
	for _, condition := range conditions {
		if condition.Status == corev1.ConditionTrue {
			taskConditions = append(taskConditions, condition)
		}
	}
	if len(taskConditions) > 0 {
		return &sortPipelineTaskStatusConditionsDesc(taskConditions)[0]
	}
	return nil
}

func sortPipelineTaskStatusConditionsDesc(taskConditions knative.Conditions) knative.Conditions {
	sort.Slice(taskConditions, func(i, j int) bool {
		if taskConditions[i].LastTransitionTime.Inner.IsZero() || taskConditions[j].LastTransitionTime.Inner.IsZero() {
			return false
		}
		return taskConditions[j].LastTransitionTime.Inner.Before(&taskConditions[i].LastTransitionTime.Inner)
	})
	return taskConditions
}

func sortPipelineTasks(tasks []jobModels.PipelineTask) []jobModels.PipelineTask {
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].Started == "" || tasks[j].Started == "" {
			return false
		}
		return tasks[i].Started < tasks[j].Started
	})
	return tasks
}

func getStepFromRadixJob(job *v1.RadixJob, stepName string) *v1.RadixJobStep {
	for _, step := range job.Status.Steps {
		if step.Name == stepName {
			return &step
		}
	}

	return nil
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

func (jh JobHandler) getAllJobs() ([]*jobModels.JobSummary, error) {
	return jh.getJobsInNamespace(corev1.NamespaceAll)
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
	allJobs, err := jh.getAllJobs()
	if err != nil {
		return nil, err
	}

	sort.Slice(allJobs, func(i, j int) bool {
		switch strings.Compare(allJobs[i].AppName, allJobs[j].AppName) {
		case -1:
			return true
		case 1:
			return false
		}

		return utils.IsBefore(allJobs[j], allJobs[i])
	})

	applicationJob := make(map[string]*jobModels.JobSummary)
	for _, job := range allJobs {
		if applicationJob[job.AppName] != nil {
			continue
		}
		if forApplications[job.AppName] != true {
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
