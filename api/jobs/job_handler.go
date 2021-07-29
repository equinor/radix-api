package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"strings"

	deployments "github.com/equinor/radix-api/api/deployments"
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	jobModels "github.com/equinor/radix-api/api/jobs/models"
	"github.com/equinor/radix-api/api/utils"
	"github.com/equinor/radix-api/models"
	radixhttp "github.com/equinor/radix-common/net/http"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
)

const workerImage = "radix-pipeline"

// RadixJobTypeJob TODO: Move this into kube, or another central location
const RadixJobTypeJob = "job"

func StepNotFoundError(stepName string) error {
	return radixhttp.NotFoundError(fmt.Sprintf("step %s not found", stepName))
}

func StepScanOutputNotDefined(stepName string) error {
	return radixhttp.NotFoundError(fmt.Sprintf("scan output for step %s not defined", stepName))
}

func StepScanOutputInvalidConfig(stepName string) error {
	return &radixhttp.Error{
		Type:    radixhttp.Server,
		Message: fmt.Sprintf("scan output configuration for step %s is invalid", stepName),
	}
}

func StepScanOutputMissingKeyInConfigMap(stepName string) error {
	return &radixhttp.Error{
		Type:    radixhttp.Server,
		Message: fmt.Sprintf("scan output data for step %s not found", stepName),
	}
}

func StepScanOutputInvalidConfigMapData(stepName string) error {
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

	jobComponents, err := jh.getJobComponents(appName, jobName, jobDeployments)
	if err != nil {
		return nil, err
	}

	return jobModels.GetJobFromRadixJob(job, jobDeployments, jobComponents), nil
}

func (jh JobHandler) GetPipelineJobStepScanOutput(appName, jobName, stepName string) ([]jobModels.Vulnerability, error) {
	namespace := crdUtils.GetAppNamespace(appName)
	job, err := jh.userAccount.RadixClient.RadixV1().RadixJobs(namespace).Get(context.TODO(), jobName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	step := getStepFromRadixJob(job, stepName)
	if step == nil {
		return nil, StepNotFoundError(stepName)
	}

	if step.Output == nil || step.Output.Scan == nil {
		return nil, StepScanOutputNotDefined(stepName)
	}

	scanOutputName, scanOutputKey := strings.TrimSpace(step.Output.Scan.VulnerabilityListConfigMap), strings.TrimSpace(step.Output.Scan.VulnerabilityListKey)
	if scanOutputName == "" || scanOutputKey == "" {
		return nil, StepScanOutputInvalidConfig(stepName)
	}

	scanOutputConfigMap, err := jh.userAccount.Client.CoreV1().ConfigMaps(namespace).Get(context.TODO(), scanOutputName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	scanOutput, found := scanOutputConfigMap.Data[scanOutputKey]
	if !found {
		return nil, StepScanOutputMissingKeyInConfigMap(stepName)
	}

	var vulnerabilities []jobModels.Vulnerability
	if err := json.Unmarshal([]byte(scanOutput), &vulnerabilities); err != nil {
		return nil, StepScanOutputInvalidConfigMapData(stepName)
	}

	return vulnerabilities, nil
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
	return jh.getJobsInNamespace(jh.serviceAccount.RadixClient, corev1.NamespaceAll)
}

func (jh JobHandler) getJobs(appName string) ([]*jobModels.JobSummary, error) {
	return jh.getJobsInNamespace(jh.userAccount.RadixClient, crdUtils.GetAppNamespace(appName))
}

func (jh JobHandler) getJobsInNamespace(radixClient radixclient.Interface, namespace string) ([]*jobModels.JobSummary, error) {
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

func (jh JobHandler) getJobComponents(appName string, jobName string, jobDeployments []*deploymentModels.DeploymentSummary) ([]*deploymentModels.ComponentSummary, error) {
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
