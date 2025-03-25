package models

import (
	"github.com/equinor/radix-common/utils/slice"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	"sort"
	"strings"
	"time"

	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

const (
	RadixPipelineJobRerunAnnotation = "radix.equinor.com/rerun-pipeline-job-from"
)

// Job holds general information about job
// swagger:model Job
type Job struct {
	// Name of the job
	//
	// required: false
	// example: radix-pipeline-20181029135644-algpv-6hznh
	Name string `json:"name"`

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
	// required: false
	// example: 2006-01-02T15:04:05Z
	Created string `json:"created"`

	// TriggeredBy user that triggered the job. If through webhook = sender.login. If through api = usertoken.upn
	//
	// required: false
	// example: a_user@equinor.com
	TriggeredBy string `json:"triggeredBy"`

	// RerunFromJob The source name of the job if this job was restarted from it
	//
	// required: false
	// example: radix-pipeline-20231011104617-urynf
	RerunFromJob string `json:"rerunFromJob"`

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
	// enum: Queued,Waiting,Running,Succeeded,Failed,Stopped,Stopping,StoppedNoChanges
	// example: Waiting
	Status string `json:"status"`

	// Name of the pipeline
	//
	// required: false
	// enum: build,build-deploy,promote,deploy,apply-config
	// example: build-deploy
	Pipeline string `json:"pipeline"`

	// RadixDeployment name, which is promoted
	//
	// required: false
	PromotedFromDeployment string `json:"promotedFromDeployment,omitempty"`

	// PromotedFromEnvironment the name of the environment that was promoted from
	//
	// required: false
	// example: dev
	PromotedFromEnvironment string `json:"promotedFromEnvironment,omitempty"`

	// PromotedToEnvironment the name of the environment that was promoted to
	//
	// required: false
	// example: qa
	PromotedToEnvironment string `json:"promotedToEnvironment,omitempty"`

	// DeployedToEnvironment the name of the environment that was deployed to
	//
	// required: false
	// example: qa
	DeployedToEnvironment string `json:"deployedToEnvironment,omitempty"`

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
	// Deprecated: Inspect each deployment to get list of components created by the job
	//
	// required: false
	// type: "array"
	// items:
	//    "$ref": "#/definitions/ComponentSummary"
	Components []*deploymentModels.ComponentSummary `json:"components,omitempty"`

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

	// TriggeredFromWebhook indicates if the job was triggered from a webhook
	//
	// required: false
	// example: true
	TriggeredFromWebhook bool `json:"triggeredFromWebhook,omitempty"`
}

// GetJobFromRadixJob Gets job from a radix job
func GetJobFromRadixJob(job *radixv1.RadixJob, jobDeployments []*deploymentModels.DeploymentSummary, runSubPipeline bool, subPipelineTaskRuns []pipelinev1.TaskRun) *Job {
	steps := getJobStepsFromRadixJob(job, runSubPipeline, subPipelineTaskRuns)

	created := radixutils.FormatTime(&job.CreationTimestamp)
	if job.Status.Created != nil {
		// Use this instead, because in a migration this may be more correct
		// as migrated jobs will have the same creation timestamp in the new cluster
		created = radixutils.FormatTime(job.Status.Created)
	}

	var jobComponents []*deploymentModels.ComponentSummary
	if len(jobDeployments) > 0 {
		jobComponents = jobDeployments[0].Components
	}

	jobModel := Job{
		Name:                 job.GetName(),
		Created:              created,
		Started:              radixutils.FormatTime(job.Status.Started),
		Ended:                radixutils.FormatTime(job.Status.Ended),
		Status:               GetStatusFromRadixJobStatus(job.Status, job.Spec.Stop),
		Pipeline:             string(job.Spec.PipeLineType),
		Steps:                steps,
		Deployments:          jobDeployments,
		Components:           jobComponents,
		TriggeredBy:          job.Spec.TriggeredBy,
		TriggeredFromWebhook: job.Spec.TriggeredFromWebhook,
		RerunFromJob:         job.Annotations[RadixPipelineJobRerunAnnotation],
	}
	switch job.Spec.PipeLineType {
	case radixv1.Build, radixv1.BuildDeploy:
		jobModel.Branch = job.Spec.Build.Branch
		jobModel.DeployedToEnvironment = job.Spec.Build.ToEnvironment
		jobModel.CommitID = job.Spec.Build.CommitID
		jobModel.OverrideUseBuildCache = job.Spec.Build.OverrideUseBuildCache
	case radixv1.Deploy:
		jobModel.ImageTagNames = job.Spec.Deploy.ImageTagNames
		jobModel.DeployedToEnvironment = job.Spec.Deploy.ToEnvironment
		jobModel.CommitID = job.Spec.Deploy.CommitID
	case radixv1.Promote:
		jobModel.PromotedFromDeployment = job.Spec.Promote.DeploymentName
		jobModel.PromotedFromEnvironment = job.Spec.Promote.FromEnvironment
		jobModel.PromotedToEnvironment = job.Spec.Promote.ToEnvironment
		jobModel.CommitID = job.Spec.Promote.CommitID
	case radixv1.ApplyConfig:
		jobModel.DeployExternalDNS = pointers.Ptr(job.Spec.ApplyConfig.DeployExternalDNS)
	}
	return &jobModel
}

func getJobStepsFromRadixJob(job *radixv1.RadixJob, runSubPipeline bool, subPipelineTaskRuns []pipelinev1.TaskRun) []Step {
	var steps []Step
	var buildSteps []Step

	for _, jobStep := range job.Status.Steps {
		step := Step{
			Name:       jobStep.Name,
			Status:     string(jobStep.Condition),
			PodName:    jobStep.PodName,
			Components: jobStep.Components,
		}
		if jobStep.Started != nil {
			step.Started = &jobStep.Started.Time
		}
		if jobStep.Ended != nil {
			step.Ended = &jobStep.Ended.Time
		}
		if strings.HasPrefix(step.Name, "build-") {
			buildSteps = append(buildSteps, step)
		} else {
			steps = append(steps, step)
		}
	}
	if runSubPipeline {
		steps = append(steps, getSubPipelineTasksSteps(subPipelineTaskRuns)...)
	}
	sort.Slice(buildSteps, func(i, j int) bool { return buildSteps[i].Name < buildSteps[j].Name })
	return append(steps, buildSteps...)
}

func getSubPipelineTasksSteps(subPipelineTaskRuns []pipelinev1.TaskRun) []Step {
	var steps []Step
	for _, run := range subPipelineTaskRuns {
		var running, failed, succeeded bool
		var started, ended *metav1.Time
		runStatus := run.Status
		if runStatus.StartTime != nil && (started == nil || runStatus.StartTime.Before(started)) {
			started = runStatus.StartTime
		}
		if runStatus.CompletionTime != nil && (ended == nil || runStatus.CompletionTime.After(ended.Time)) {
			ended = runStatus.CompletionTime
		}
		if runStatus.CompletionTime != nil {
			succeeded = true
		} else if runStatus.StartTime != nil {
			running = true
		} else if slice.Any(run.Status.Conditions, func(condition apis.Condition) bool { return condition.Reason == string(PipelineRunReasonFailed) }) {
			failed = true
		}
		status := radixv1.JobWaiting
		if failed {
			status = radixv1.JobFailed
		} else if running {
			status = radixv1.JobRunning
		} else if succeeded {
			status = radixv1.JobSucceeded
		}
		var startedTime, endedTime *time.Time
		if started != nil {
			startedTime = &started.Time
		}
		if ended != nil {
			endedTime = &ended.Time
		}
		steps = append(steps, Step{
			Name:       run.Name,
			Status:     string(status),
			Started:    startedTime,
			Ended:      endedTime,
			PodName:    run.Status.PodName,
			Components: nil,
		})
	}
	return steps
}
