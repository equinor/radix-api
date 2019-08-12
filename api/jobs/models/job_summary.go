package models

import (
	"github.com/equinor/radix-api/api/utils"
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
	// Enum: build-deploy, build
	// example: build-deploy
	Pipeline string `json:"pipeline"`

	// Environments the job deployed to
	//
	// required: false
	// example: dev,qa
	Environments []string `json:"environments,omitempty"`
}

// GetJobSummary Used to get job summary from a kubernetes job
func GetJobSummary(job *batchv1.Job) *JobSummary {
	appName := job.Labels[kube.RadixAppLabel]
	branch := getBranchFromAnnotation(job)
	commit := job.Labels[kube.RadixCommitLabel]

	// TODO: Move string into constant
	pipeline := job.Labels["radix-pipeline"]

	status := job.Status

	jobStatus := GetStatusFromJobStatus(status)
	ended := utils.FormatTime(status.CompletionTime)
	if jobStatus == Failed {
		ended = utils.FormatTime(&status.Conditions[0].LastTransitionTime)
	}

	pipelineJob := &JobSummary{
		Name:     job.Name,
		AppName:  appName,
		Branch:   branch,
		CommitID: commit,
		Status:   jobStatus.String(),
		Started:  utils.FormatTime(status.StartTime),
		Ended:    ended,
		Pipeline: pipeline,
	}
	return pipelineJob
}

// GetSummaryFromRadixJob Used to get job summary from a radix job
func GetSummaryFromRadixJob(job *v1.RadixJob) *JobSummary {
	status := job.Status

	jobStatus := status.Condition
	ended := utils.FormatTime(status.Ended)

	pipelineJob := &JobSummary{
		Name:         job.Name,
		AppName:      job.Spec.AppName,
		Branch:       job.Spec.Build.Branch,
		CommitID:     job.Spec.Build.CommitID,
		Status:       string(jobStatus),
		Started:      utils.FormatTime(status.Started),
		Ended:        ended,
		Pipeline:     string(job.Spec.PipeLineType),
		Environments: job.Status.TargetEnvs,
	}

	return pipelineJob
}

func getBranchFromAnnotation(job *batchv1.Job) string {
	if len(job.Annotations) > 0 && job.Annotations[kube.RadixBranchAnnotation] != "" {
		return job.Annotations[kube.RadixBranchAnnotation]
	}

	return job.Labels[kube.RadixBranchDeprecated]
}
