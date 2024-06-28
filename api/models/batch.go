package models

import (
	"fmt"

	deploymentmodels "github.com/equinor/radix-api/api/deployments/models"
	"github.com/equinor/radix-api/api/utils/predicate"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
)

func BuildScheduledBatchSummaries(rbList []radixv1.RadixBatch, rdRefs []radixv1.RadixDeployment) []deploymentmodels.ScheduledBatchSummary {
	batchSummaries := make([]deploymentmodels.ScheduledBatchSummary, 0, len(rbList))

	for _, batch := range rbList {
		var rdRef *radixv1.RadixDeployment
		if rd, found := slice.FindFirst(rdRefs, predicate.IsRadixDeploymentForRadixBatch(&batch)); found {
			rdRef = &rd
		}

		batchSummaries = append(batchSummaries, *BuildScheduledBatchSummary(&batch, rdRef))
	}

	return batchSummaries
}

func BuildScheduledBatchSummary(rb *radixv1.RadixBatch, rdRef *radixv1.RadixDeployment) *deploymentmodels.ScheduledBatchSummary {
	batchSummary := &deploymentmodels.ScheduledBatchSummary{
		Name:           rb.Name,
		DeploymentName: rb.Spec.RadixDeploymentJobRef.Name,
		Status:         string(getScheduledBatchStatus(rb)),
		JobList:        buildScheduledJobSummaries(rb, rdRef),
		TotalJobCount:  len(rb.Spec.Jobs),
		Created:        radixutils.FormatTimestamp(rb.GetCreationTimestamp().Time),
		Started:        radixutils.FormatTime(rb.Status.Condition.ActiveTime),
		Ended:          radixutils.FormatTime(rb.Status.Condition.CompletionTime),
	}

	return batchSummary
}

func buildScheduledJobSummaries(rb *radixv1.RadixBatch, rdRef *radixv1.RadixDeployment) []deploymentmodels.ScheduledJobSummary {
	jobSummaries := make([]deploymentmodels.ScheduledJobSummary, 0, len(rb.Spec.Jobs))

	for i := range rb.Spec.Jobs {
		jobSummaries = append(jobSummaries, *buildScheduledJobSummary(rb, i, rdRef))
	}

	return jobSummaries
}

func buildScheduledJobSummary(rb *radixv1.RadixBatch, jobIndex int, rdRef *radixv1.RadixDeployment) *deploymentmodels.ScheduledJobSummary {
	var batchName string
	if rb.GetLabels()[kube.RadixBatchTypeLabel] == string(kube.RadixBatchTypeBatch) {
		batchName = rb.GetName()
	}

	job := rb.Spec.Jobs[jobIndex]

	summary := deploymentmodels.ScheduledJobSummary{
		Name:           fmt.Sprintf("%s-%s", rb.GetName(), job.Name),
		DeploymentName: rb.Spec.RadixDeploymentJobRef.Name,
		BatchName:      batchName,
		JobId:          job.JobId,
		ReplicaList:    getBatchJobReplicaSummaries(rb, job),
		Status:         radixv1.RadixBatchJobApiStatusWaiting,
	}

	var jobComponent *radixv1.RadixDeployJobComponent
	if rdRef != nil && predicate.IsRadixDeploymentForRadixBatch(rb)(*rdRef) {
		jobComponent = rdRef.GetJobComponentByName(rb.Spec.RadixDeploymentJobRef.Job)
	}

	if jobComponent != nil {
		summary.Runtime = &deploymentmodels.Runtime{
			Architecture: operatorutils.GetArchitectureFromRuntime(jobComponent.GetRuntime()),
		}

		summary.TimeLimitSeconds = jobComponent.TimeLimitSeconds
		if job.TimeLimitSeconds != nil {
			summary.TimeLimitSeconds = job.TimeLimitSeconds
		}

		if jobComponent.BackoffLimit != nil {
			summary.BackoffLimit = *jobComponent.BackoffLimit
		}
		if job.BackoffLimit != nil {
			summary.BackoffLimit = *job.BackoffLimit
		}

		if jobComponent.Node != (radixv1.RadixNode{}) {
			summary.Node = (*deploymentmodels.Node)(&jobComponent.Node)
		}
		if job.Node != nil {
			summary.Node = (*deploymentmodels.Node)(job.Node)
		}

		if job.Resources != nil {
			summary.Resources = deploymentmodels.ConvertRadixResourceRequirements(*job.Resources)
		} else if len(jobComponent.Resources.Requests) > 0 || len(jobComponent.Resources.Limits) > 0 {
			summary.Resources = deploymentmodels.ConvertRadixResourceRequirements(jobComponent.Resources)
		}
	}

	stopJob := job.Stop != nil && *job.Stop

	if status, found := slice.FindFirst(rb.Status.JobStatuses, predicate.IsBatchJobStatusForBatchJob(job)); found {
		summary.Status = string(getBatchJobSummaryStatus(status, stopJob))
		summary.Created = radixutils.FormatTime(status.CreationTime)
		summary.Started = radixutils.FormatTime(status.StartTime)
		summary.Ended = radixutils.FormatTime(status.EndTime)
		summary.Message = status.Message
		summary.FailedCount = status.Failed
		summary.Restart = status.Restart
	} else if stopJob {
		summary.Status = radixv1.RadixBatchJobApiStatusStopping
	}
	return &summary
}

func getScheduledBatchStatus(batch *radixv1.RadixBatch) radixv1.RadixBatchJobApiStatus {
	switch {
	case batch.Status.Condition.Type == radixv1.BatchConditionTypeActive:
		if slice.Any(batch.Status.JobStatuses, func(jobStatus radixv1.RadixBatchJobStatus) bool {
			return jobStatus.Phase == radixv1.BatchJobPhaseRunning
		}) {
			return radixv1.RadixBatchJobApiStatusRunning
		}
		return radixv1.RadixBatchJobApiStatusActive
	case batch.Status.Condition.Type == radixv1.BatchConditionTypeCompleted:
		if len(batch.Status.JobStatuses) > 0 && slice.All(batch.Status.JobStatuses, func(jobStatus radixv1.RadixBatchJobStatus) bool {
			return jobStatus.Phase == radixv1.BatchJobPhaseFailed
		}) {
			return radixv1.RadixBatchJobApiStatusFailed
		}
		return radixv1.RadixBatchJobApiStatusSucceeded
	}
	return radixv1.RadixBatchJobApiStatusWaiting
}

func getBatchJobSummaryStatus(jobStatus radixv1.RadixBatchJobStatus, stopJob bool) (status radixv1.RadixBatchJobApiStatus) {
	status = radixv1.RadixBatchJobApiStatusWaiting
	switch jobStatus.Phase {
	case radixv1.BatchJobPhaseActive:
		status = radixv1.RadixBatchJobApiStatusActive
	case radixv1.BatchJobPhaseRunning:
		status = radixv1.RadixBatchJobApiStatusRunning
	case radixv1.BatchJobPhaseSucceeded:
		status = radixv1.RadixBatchJobApiStatusSucceeded
	case radixv1.BatchJobPhaseFailed:
		status = radixv1.RadixBatchJobApiStatusFailed
	case radixv1.BatchJobPhaseStopped:
		status = radixv1.RadixBatchJobApiStatusStopped
	case radixv1.BatchJobPhaseWaiting:
		status = radixv1.RadixBatchJobApiStatusWaiting
	}
	if stopJob && (status == radixv1.RadixBatchJobApiStatusWaiting || status == radixv1.RadixBatchJobApiStatusActive || status == radixv1.RadixBatchJobApiStatusRunning) {
		return radixv1.RadixBatchJobApiStatusStopping
	}
	if len(jobStatus.RadixBatchJobPodStatuses) > 0 && slice.All(jobStatus.RadixBatchJobPodStatuses, func(jobPodStatus radixv1.RadixBatchJobPodStatus) bool {
		return jobPodStatus.Phase == radixv1.PodFailed
	}) {
		return radixv1.RadixBatchJobApiStatusFailed
	}
	return status
}

func getBatchJobReplicaSummaries(radixBatch *radixv1.RadixBatch, job radixv1.RadixBatchJob) []deploymentmodels.ReplicaSummary {
	if jobStatus, ok := slice.FindFirst(radixBatch.Status.JobStatuses, func(jobStatus radixv1.RadixBatchJobStatus) bool {
		return jobStatus.Name == job.Name
	}); ok {
		return slice.Map(jobStatus.RadixBatchJobPodStatuses, func(status radixv1.RadixBatchJobPodStatus) deploymentmodels.ReplicaSummary {
			return getBatchJobReplicaSummary(status, job)
		})
	}
	return nil
}

func getBatchJobReplicaSummary(status radixv1.RadixBatchJobPodStatus, job radixv1.RadixBatchJob) deploymentmodels.ReplicaSummary {
	summary := deploymentmodels.ReplicaSummary{
		Name:          status.Name,
		Created:       radixutils.FormatTimestamp(status.CreationTime.Time),
		RestartCount:  status.RestartCount,
		Image:         status.Image,
		ImageId:       status.ImageID,
		PodIndex:      status.PodIndex,
		Reason:        status.Reason,
		StatusMessage: status.Message,
		ExitCode:      status.ExitCode,
		Status:        getBatchJobReplicaSummaryStatus(status),
	}
	if status.StartTime != nil {
		summary.StartTime = radixutils.FormatTimestamp(status.StartTime.Time)
	}
	if status.EndTime != nil {
		summary.EndTime = radixutils.FormatTimestamp(status.EndTime.Time)
	}
	if job.Resources != nil {
		summary.Resources = pointers.Ptr(deploymentmodels.ConvertRadixResourceRequirements(*job.Resources))
	}
	return summary
}

func getBatchJobReplicaSummaryStatus(status radixv1.RadixBatchJobPodStatus) deploymentmodels.ReplicaStatus {
	replicaStatus := deploymentmodels.ReplicaStatus{}
	switch status.Phase {
	case radixv1.PodFailed:
		replicaStatus.Status = "Failed"
	case radixv1.PodRunning:
		replicaStatus.Status = "Running"
	case radixv1.PodSucceeded:
		replicaStatus.Status = "Succeeded"
	default:
		replicaStatus.Status = "Pending"
	}
	return replicaStatus
}
