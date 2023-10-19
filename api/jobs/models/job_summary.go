package models

import (
	radixutils "github.com/equinor/radix-common/utils"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// JobSummary holds general information about job
// swagger:model JobSummary
type JobSummary struct {
	// Name of the job
	//
	// required: false
	// example: radix-pipeline-20181029135644-algpv-6hznh
	Name string `json:"name"`

	// AppName of the application
	//
	// required: false
	// example: radix-pipeline-20181029135644-algpv-6hznh
	AppName string `json:"appName"`

	// Branch to build from
	//
	// required: false
	// example: master
	Branch string `json:"branch"`

	// CommitID the commit ID of the branch to build
	//
	// required: false
	// example: 4faca8595c5283a9d0f17a623b9255a0d9866a2e
	CommitID string `json:"commitID"`

	// Image tags names for components - if empty will use default logic
	//
	// required: false
	// Example: component1: tag1,component2: tag2
	ImageTagNames map[string]string `json:"imageTagNames,omitempty"`

	// Created timestamp
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	Created string `json:"created"`

	// TriggeredBy user that triggered the job. If through webhook = sender.login. If through api - usertoken.upn
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
	// Enum: Waiting,Running,Succeeded,Stopping,Stopped,Failed,StoppedNoChanges
	// example: Waiting
	Status string `json:"status"`

	// Name of the pipeline
	//
	// required: false
	// Enum: build-deploy, build
	// example: build-deploy
	Pipeline string `json:"pipeline"`

	// Environments the job deployed to
	//
	// required: false
	// example: ["dev", "qa"]
	Environments []string `json:"environments,omitempty"`

	// RadixDeployment name, which is promoted
	//
	// required: false
	PromotedFromDeployment string `json:"promotedFromDeployment,omitempty"`

	// Environment name, from which the Radix deployment is promoted
	//
	// required: false
	PromotedFromEnvironment string `json:"promotedFromEnvironment,omitempty"`

	// Environment name, to which the Radix deployment is promoted
	//
	// required: false
	PromotedToEnvironment string `json:"promotedToEnvironment,omitempty"`
}

// GetSummaryFromRadixJob Used to get job summary from a radix job
func GetSummaryFromRadixJob(job *radixv1.RadixJob) *JobSummary {
	status := job.Status
	ended := radixutils.FormatTime(status.Ended)
	created := radixutils.FormatTime(&job.CreationTimestamp)
	if status.Created != nil {
		// Use this instead, because in a migration this may be more correct
		// as migrated jobs will have the same creation timestamp in the new cluster
		created = radixutils.FormatTime(status.Created)
	}

	pipelineJob := &JobSummary{
		Name:         job.Name,
		AppName:      job.Spec.AppName,
		Status:       GetStatusFromRadixJobStatus(status, job.Spec.Stop),
		Created:      created,
		Started:      radixutils.FormatTime(status.Started),
		Ended:        ended,
		Pipeline:     string(job.Spec.PipeLineType),
		Environments: job.Status.TargetEnvs,
		TriggeredBy:  job.Spec.TriggeredBy,
	}
	switch job.Spec.PipeLineType {
	case radixv1.Build, radixv1.BuildDeploy:
		pipelineJob.Branch = job.Spec.Build.Branch
		pipelineJob.CommitID = job.Spec.Build.CommitID
	case radixv1.Deploy:
		pipelineJob.ImageTagNames = job.Spec.Deploy.ImageTagNames
		pipelineJob.CommitID = job.Spec.Deploy.CommitID
	case radixv1.Promote:
		pipelineJob.PromotedFromDeployment = job.Spec.Promote.DeploymentName
		pipelineJob.PromotedFromEnvironment = job.Spec.Promote.FromEnvironment
		pipelineJob.PromotedToEnvironment = job.Spec.Promote.ToEnvironment
		pipelineJob.CommitID = job.Spec.Promote.CommitID
	}

	return pipelineJob
}

func (job *JobSummary) GetCreated() string {
	return job.Created
}

func (job *JobSummary) GetStarted() string {
	return job.Started
}

func (job *JobSummary) GetEnded() string {
	return job.Ended
}

func (job *JobSummary) GetStatus() string {
	return job.Status
}
