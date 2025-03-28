package models

import (
	"time"

	"github.com/equinor/radix-common/utils/pointers"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// JobSummary holds general information about job
// swagger:model JobSummary
type JobSummary struct {
	// Name of the job
	//
	// required: true
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
	// example: component1: tag1,component2: tag2
	ImageTagNames map[string]string `json:"imageTagNames,omitempty"`

	// Created timestamp
	//
	// required: true
	// swagger:strfmt date-time
	Created time.Time `json:"created"`

	// TriggeredBy user that triggered the job. If through webhook = sender.login. If through api - usertoken.upn
	//
	// required: false
	// example: a_user@equinor.com
	TriggeredBy string `json:"triggeredBy"`

	// Started timestamp
	//
	// required: false
	// swagger:strfmt date-time
	Started *time.Time `json:"started"`

	// Ended timestamp
	//
	// required: false
	// swagger:strfmt date-time
	Ended *time.Time `json:"ended"`

	// Status of the job
	//
	// required: false
	// enum: Queued,Waiting,Running,Succeeded,Failed,Stopped,Stopping,StoppedNoChanges
	// example: Waiting
	Status string `json:"status"`

	// Name of the pipeline
	//
	// required: false
	// enum: build,build-deploy,promote,deploy,apply-config
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

	// OverrideUseBuildCache override default or configured build cache option
	//
	// required: false
	// Extensions:
	// x-nullable: true
	OverrideUseBuildCache *bool `json:"overrideUseBuildCache,omitempty"`

	// DeployExternalDNS deploy external DNS
	//
	// required: false
	// Extensions:
	// x-nullable: true
	DeployExternalDNS *bool `json:"deployExternalDNS,omitempty"`
}

// GetSummaryFromRadixJob Used to get job summary from a radix job
func GetSummaryFromRadixJob(job *radixv1.RadixJob) *JobSummary {
	created := job.CreationTimestamp.Time
	if job.Status.Created != nil {
		// Use this instead, because in a migration this may be more correct
		// as migrated jobs will have the same creation timestamp in the new cluster
		created = job.Status.Created.Time
	}

	var started, ended *time.Time
	if job.Status.Started != nil {
		started = &job.Status.Started.Time
	}
	if job.Status.Ended != nil {
		ended = &job.Status.Ended.Time
	}

	pipelineJob := &JobSummary{
		Name:         job.Name,
		AppName:      job.Spec.AppName,
		Status:       GetStatusFromRadixJobStatus(job.Status, job.Spec.Stop),
		Created:      created,
		Started:      started,
		Ended:        ended,
		Pipeline:     string(job.Spec.PipeLineType),
		Environments: job.Status.TargetEnvs,
		TriggeredBy:  job.Spec.TriggeredBy,
	}
	switch job.Spec.PipeLineType {
	case radixv1.Build, radixv1.BuildDeploy:
		pipelineJob.Branch = job.Spec.Build.Branch
		pipelineJob.CommitID = job.Spec.Build.CommitID
		pipelineJob.OverrideUseBuildCache = job.Spec.Build.OverrideUseBuildCache
	case radixv1.Deploy:
		pipelineJob.ImageTagNames = job.Spec.Deploy.ImageTagNames
		pipelineJob.CommitID = job.Spec.Deploy.CommitID
	case radixv1.Promote:
		pipelineJob.PromotedFromDeployment = job.Spec.Promote.DeploymentName
		pipelineJob.PromotedFromEnvironment = job.Spec.Promote.FromEnvironment
		pipelineJob.PromotedToEnvironment = job.Spec.Promote.ToEnvironment
		pipelineJob.CommitID = job.Spec.Promote.CommitID
	case radixv1.ApplyConfig:
		pipelineJob.DeployExternalDNS = pointers.Ptr(job.Spec.ApplyConfig.DeployExternalDNS)
	}

	return pipelineJob
}

func (job *JobSummary) GetCreated() *time.Time {
	if job.Created.IsZero() {
		return nil
	}

	return &job.Created
}

func (job *JobSummary) GetStarted() *time.Time {
	return job.Started
}

func (job *JobSummary) GetEnded() *time.Time {
	return job.Ended
}

func (job *JobSummary) GetStatus() string {
	return job.Status
}
