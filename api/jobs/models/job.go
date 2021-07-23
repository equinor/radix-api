package models

import (
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	radixutils "github.com/equinor/radix-common/utils"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
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

	// Created timestamp
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	Created string `json:"created"`

	// TriggeredBy user that triggered the job. If through webhook = sender.login. If through api = usertoken.upn
	//
	// required: false
	// example: a_user@equinor.com
	TriggeredBy string `json:"triggeredBy"`

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
	// Enum: Waiting,Running,Succeeded,Stopping,Stopped,Failed
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

// GetJobFromRadixJob Gets job from a radix job
func GetJobFromRadixJob(job *v1.RadixJob, jobDeployments []*deploymentModels.DeploymentSummary, jobComponents []*deploymentModels.ComponentSummary) *Job {
	steps := GetJobStepsFromRadixJob(job)

	created := radixutils.FormatTime(&job.CreationTimestamp)
	if job.Status.Created != nil {
		// Use this instead, because in a migration this may be more correct
		// as migrated jobs will have the same creation timestamp in the new cluster
		created = radixutils.FormatTime(job.Status.Created)
	}

	return &Job{
		Name:        job.GetName(),
		Branch:      job.Spec.Build.Branch,
		CommitID:    job.Spec.Build.CommitID,
		Created:     created,
		Started:     radixutils.FormatTime(job.Status.Started),
		Ended:       radixutils.FormatTime(job.Status.Ended),
		Status:      GetStatusFromRadixJobStatus(job.Status, job.Spec.Stop),
		Pipeline:    string(job.Spec.PipeLineType),
		Steps:       steps,
		Deployments: jobDeployments,
		Components:  jobComponents,
		TriggeredBy: job.Spec.TriggeredBy,
	}
}

// GetJobStepsFromRadixJob Gets the steps from a Radix job
func GetJobStepsFromRadixJob(job *v1.RadixJob) []Step {
	var steps []Step
	for _, jobStep := range job.Status.Steps {
		step := Step{
			Name:              jobStep.Name,
			Status:            string(jobStep.Condition),
			Started:           radixutils.FormatTime(jobStep.Started),
			Ended:             radixutils.FormatTime(jobStep.Ended),
			PodName:           jobStep.PodName,
			Components:        jobStep.Components,
			VulnerabilityScan: GetVulnerabilityScanFromRadixJobStep(jobStep),
		}

		steps = append(steps, step)
	}

	return steps
}

func GetVulnerabilityScanFromRadixJobStep(step v1.RadixJobStep) *VulnerabilityScan {
	if step.Output == nil || step.Output.Scan == nil {
		return nil
	}

	return &VulnerabilityScan{
		Status:          step.Output.Scan.Status,
		Reason:          step.Output.Scan.Reason,
		Vulnerabilities: step.Output.Scan.Vulnerabilities,
	}
}
