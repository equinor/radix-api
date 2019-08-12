package models

import (
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	"github.com/equinor/radix-api/api/utils"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Job holds general information about job
// swagger:model Job
type Job struct {
	// Name of the job
	//
	// required: false
	// example: radix-pipeline-20181029135644-algpv-6hznh
	Name string `json:"name"`

	// Branch branch to build from
	//
	// required: false
	// example: master
	Branch string `json:"branch"`

	// CommitID the commit ID of the branch to build
	//
	// required: false
	// example: 4faca8595c5283a9d0f17a623b9255a0d9866a2e
	CommitID string `json:"commitID"`

	// Started timestamp
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	Started string `json:"started"`

	// Ended timestamp
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	Ended string `json:"ended"`

	// Status of the job
	//
	// required: false
	// Enum: Waiting,Running,Succeeded,Failed
	// example: Waiting
	Status string `json:"status"`

	// Name of the pipeline
	//
	// required: false
	// Enum: build-deploy
	// example: build-deploy
	Pipeline string `json:"pipeline"`

	// Array of steps
	//
	// required: false
	// type: "array"
	// items:
	//    "$ref": "#/definitions/Step"
	Steps []Step `json:"steps"`

	// Array of deployments
	//
	// required: false
	// type: "array"
	// items:
	//    "$ref": "#/definitions/DeploymentSummary"
	Deployments []*deploymentModels.DeploymentSummary `json:"deployments,omitempty"`

	// Components (array of ComponentSummary) created by the job
	//
	// required: false
	// type: "array"
	// items:
	//    "$ref": "#/definitions/ComponentSummary"
	Components []*deploymentModels.ComponentSummary `json:"components,omitempty"`
}

// GetJob Transforms kubernetes job into job details
func GetJob(job *batchv1.Job, steps []Step, jobDeployments []*deploymentModels.DeploymentSummary, jobComponents []*deploymentModels.ComponentSummary) *Job {
	jobStatus := GetStatusFromJobStatus(job.Status)
	var jobEnded metav1.Time

	if len(job.Status.Conditions) > 0 {
		jobEnded = job.Status.Conditions[0].LastTransitionTime
	}

	return &Job{
		Name:        job.GetName(),
		Branch:      getBranchFromAnnotation(job),
		CommitID:    job.Labels[kube.RadixCommitLabel],
		Started:     utils.FormatTime(job.Status.StartTime),
		Ended:       utils.FormatTime(&jobEnded),
		Status:      jobStatus.String(),
		Pipeline:    job.Labels["radix-pipeline"],
		Steps:       steps,
		Deployments: jobDeployments,
		Components:  jobComponents,
	}
}

// GetJobFromRadixJob Gets job from a radix job
func GetJobFromRadixJob(job *v1.RadixJob, jobDeployments []*deploymentModels.DeploymentSummary, jobComponents []*deploymentModels.ComponentSummary) *Job {
	var steps []Step
	for _, jobStep := range job.Status.Steps {
		step := Step{
			Name:    jobStep.Name,
			Status:  string(jobStep.Condition),
			Started: utils.FormatTime(jobStep.Started),
			Ended:   utils.FormatTime(jobStep.Ended),
			PodName: jobStep.PodName,
		}

		steps = append(steps, step)
	}

	return &Job{
		Name:        job.GetName(),
		Branch:      job.Spec.Build.Branch,
		CommitID:    job.Spec.Build.CommitID,
		Started:     utils.FormatTime(job.Status.Started),
		Ended:       utils.FormatTime(job.Status.Ended),
		Status:      string(job.Status.Condition),
		Pipeline:    string(job.Spec.PipeLineType),
		Steps:       steps,
		Deployments: jobDeployments,
		Components:  jobComponents,
	}
}
