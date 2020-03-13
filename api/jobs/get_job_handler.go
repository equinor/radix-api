package jobs

import (
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"strings"

	deployments "github.com/equinor/radix-api/api/deployments"
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	jobModels "github.com/equinor/radix-api/api/jobs/models"
	"github.com/equinor/radix-api/api/utils"
	"github.com/equinor/radix-api/models"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
)

const workerImage = "radix-pipeline"

// RadixJobTypeJob TODO: Move this into kube, or another central location
const RadixJobTypeJob = "job"

// JobHandler Instance variables
type JobHandler struct {
	accounts       models.Accounts
	userAccount    models.Account
	serviceAccount models.Account
	deploy         deployments.DeployHandler
}

// Init Constructor
func Init(accounts models.Accounts) JobHandler {
	// todo! accoutn for running deploy?
	deploy := deployments.Init(accounts)

	return JobHandler{
		accounts:       accounts,
		userAccount:    accounts.UserAccount,
		serviceAccount: accounts.ServiceAccount,
		deploy:         deploy,
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
	job, err := jh.userAccount.RadixClient.RadixV1().RadixJobs(crdUtils.GetAppNamespace(appName)).Get(jobName, metav1.GetOptions{})
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

	return jobModels.GetJobFromRadixJob(job, jobDeployments, jobComponents), nil
}

func (jh JobHandler) getApplicationJobs(appName string) ([]*jobModels.JobSummary, error) {
	jobs, err := jh.getJobs(appName)
	if err != nil {
		return nil, err
	}

	// Sort jobs descending
	sort.Slice(jobs, func(i, j int) bool {
		return IsBefore(jobs[j], jobs[i])
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
	jobList, err := jh.userAccount.RadixClient.RadixV1().RadixJobs(namespace).List(metav1.ListOptions{})
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

		return IsBefore(allJobs[j], allJobs[i])
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

func (jh JobHandler) getJobComponents(appName string, jobName string) ([]*deploymentModels.ComponentSummary, error) {
	jobDeployments, err := jh.deploy.GetDeploymentsForJob(appName, jobName)
	if err != nil {
		return nil, err
	}

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
				Image: component.Image,
			}
			jobComponents = append(jobComponents, &componentSummary)
		}
	}

	return jobComponents, nil
}

// IsBefore Checks that job-j is before job-i
func IsBefore(j, i *jobModels.JobSummary) bool {
	jCreated := getTimeFromTimestamp(j.Created)
	if jCreated == nil {
		return false
	}

	iCreated := getTimeFromTimestamp(i.Created)
	if iCreated == nil {
		return true
	}

	jStarted := getTimeFromTimestamp(j.Started)
	iStarted := getTimeFromTimestamp(i.Started)

	return (jCreated.Equal(*iCreated) && jStarted != nil && iStarted != nil && jStarted.Before(*iStarted)) ||
		jCreated.Before(*iCreated)
}

func getTimeFromTimestamp(timestamp string) *time.Time {
	t, err := utils.ParseTimestamp(timestamp)
	if err != nil {
		return nil
	}

	return &t
}
