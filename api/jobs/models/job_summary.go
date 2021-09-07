package models

import (
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	batchv1 "k8s.io/api/batch/v1"
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
	// Enum: Waiting,Running,Succeeded,Stopping,Stopped,Failed
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
	// example: dev,qa
	Environments []string `json:"environments,omitempty"`

	// List of RadixJobStepScanOutput for a job
	//
	// required: false
	StepSummaryScans []v1.RadixJobStepScanOutput `json:"stepSummaryScans,omitempty"`
}

// GetStepSummaryScansFromRadixJob Used to get a list of summary scans of all steps on a job
func GetStepSummaryScansFromRadixJob(steps []v1.RadixJobStep) []v1.RadixJobStepScanOutput {
	var scans []v1.RadixJobStepScanOutput
	for _, step := range steps {
		if step.Output != nil && step.Output.Scan != nil {
			scans = append(scans, *step.Output.Scan)
		}
	}

	return scans
}

// GetSummaryFromRadixJob Used to get job summary from a radix job
func GetSummaryFromRadixJob(job *v1.RadixJob) *JobSummary {
	status := job.Status
	ended := radixutils.FormatTime(status.Ended)
	created := radixutils.FormatTime(&job.CreationTimestamp)
	if status.Created != nil {
		// Use this instead, because in a migration this may be more correct
		// as migrated jobs will have the same creation timestamp in the new cluster
		created = radixutils.FormatTime(status.Created)
	}

	pipelineJob := &JobSummary{
		Name:             job.Name,
		AppName:          job.Spec.AppName,
		Branch:           job.Spec.Build.Branch,
		CommitID:         job.Spec.Build.CommitID,
		Status:           GetStatusFromRadixJobStatus(status, job.Spec.Stop),
		Created:          created,
		Started:          radixutils.FormatTime(status.Started),
		Ended:            ended,
		Pipeline:         string(job.Spec.PipeLineType),
		Environments:     job.Status.TargetEnvs,
		TriggeredBy:      job.Spec.TriggeredBy,
		StepSummaryScans: GetStepSummaryScansFromRadixJob(status.Steps),
	}

	return pipelineJob
}

func getBranchFromAnnotation(job *batchv1.Job) string {
	if len(job.Annotations) > 0 && job.Annotations[kube.RadixBranchAnnotation] != "" {
		return job.Annotations[kube.RadixBranchAnnotation]
	}

	return job.Labels[kube.RadixBranchDeprecated]
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
