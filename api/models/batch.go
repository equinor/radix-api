package models

import (
	"fmt"

	deploymentmodels "github.com/equinor/radix-api/api/deployments/models"
	"github.com/equinor/radix-api/api/utils/predicate"
	"github.com/equinor/radix-common/utils/slice"
	jobschedulermodels "github.com/equinor/radix-job-scheduler/models/v2"
	jobschedulerbatch "github.com/equinor/radix-job-scheduler/pkg/batch"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
)

func BuildScheduledBatchSummary(radixBatch *radixv1.RadixBatch, rdList []radixv1.RadixDeployment) *deploymentmodels.ScheduledBatchSummary {
	var activeRd, batchRd *radixv1.RadixDeployment
	if rd, ok := slice.FindFirst(rdList, predicate.MatchAll(predicate.IsRadixDeploymentInNamespace(radixBatch.Namespace), predicate.IsActiveRadixDeployment)); ok {
		activeRd = &rd
	}
	if rd, ok := slice.FindFirst(rdList, predicate.IsRadixDeploymentForRadixBatch(radixBatch)); ok {
		batchRd = &rd
	}
	var activeDeployJobComponent *radixv1.RadixDeployJobComponent
	if activeRd != nil {
		if jc, ok := slice.FindFirst(activeRd.Spec.Jobs, predicate.IsRadixDeployJobComponentWithName(radixBatch.Spec.RadixDeploymentJobRef.Job)); ok {
			activeDeployJobComponent = &jc
		}
	}
	jobSchedulerBatchStatus := jobschedulerbatch.GetRadixBatchStatus(radixBatch, activeDeployJobComponent)
	scheduleJobSummaryMapper := func(job radixv1.RadixBatchJob) deploymentmodels.ScheduledJobSummary {
		return buildScheduleJobSummary(radixBatch, &job, batchRd, activeRd)
	}
	batchSummary := deploymentmodels.ScheduledBatchSummary{
		Name:           radixBatch.Name,
		BatchId:        radixBatch.Spec.BatchId,
		TotalJobCount:  len(radixBatch.Spec.Jobs),
		DeploymentName: radixBatch.Spec.RadixDeploymentJobRef.Name,
		JobList:        slice.Map(radixBatch.Spec.Jobs, scheduleJobSummaryMapper),
		Created:        &jobSchedulerBatchStatus.CreationTime,
		Started:        jobSchedulerBatchStatus.Started,
		Ended:          jobSchedulerBatchStatus.Ended,
		Status:         mapScheduledBatchJobStatus(jobSchedulerBatchStatus.Status),
	}

	return &batchSummary
}

func buildScheduleJobSummary(radixBatch *radixv1.RadixBatch, radixBatchJob *radixv1.RadixBatchJob, batchRd, activeRd *radixv1.RadixDeployment) deploymentmodels.ScheduledJobSummary {
	jobSummary := deploymentmodels.ScheduledJobSummary{
		Name:           fmt.Sprintf("%s-%s", radixBatch.GetName(), radixBatchJob.Name),
		DeploymentName: radixBatch.Spec.RadixDeploymentJobRef.Name,
		JobId:          radixBatchJob.JobId,
		Status:         deploymentmodels.ScheduledBatchJobStatusWaiting,
	}

	if radixBatch.GetLabels()[kube.RadixBatchTypeLabel] == string(kube.RadixBatchTypeBatch) {
		jobSummary.BatchName = radixBatch.GetName()
	}

	if jobStatus, ok := slice.FindFirst(radixBatch.Status.JobStatuses, predicate.IsBatchJobStatusForJobName(radixBatchJob.Name)); ok {
		jobSummary.ReplicaList = buildReplicaSummaryListFromBatchJobStatus(jobStatus)
	}

	var batchRdJobComponent *radixv1.RadixDeployJobComponent
	if batchRd != nil {
		if jc, ok := slice.FindFirst(batchRd.Spec.Jobs, predicate.IsRadixDeployJobComponentWithName(radixBatch.Spec.RadixDeploymentJobRef.Job)); ok {
			batchRdJobComponent = &jc
		}
	}
	if batchRdJobComponent != nil {
		jobSummary.Runtime = &deploymentmodels.Runtime{
			Architecture: operatorutils.GetArchitectureFromRuntime(batchRdJobComponent.GetRuntime()),
		}

		jobSummary.TimeLimitSeconds = batchRdJobComponent.TimeLimitSeconds
		if radixBatchJob.TimeLimitSeconds != nil {
			jobSummary.TimeLimitSeconds = radixBatchJob.TimeLimitSeconds
		}

		if batchRdJobComponent.BackoffLimit != nil {
			jobSummary.BackoffLimit = *batchRdJobComponent.BackoffLimit
		}
		if radixBatchJob.BackoffLimit != nil {
			jobSummary.BackoffLimit = *radixBatchJob.BackoffLimit
		}

		if batchRdJobComponent.Node != (radixv1.RadixNode{}) {
			jobSummary.Node = (*deploymentmodels.Node)(&batchRdJobComponent.Node)
		}
		if radixBatchJob.Node != nil {
			jobSummary.Node = (*deploymentmodels.Node)(radixBatchJob.Node)
		}

		if radixBatchJob.Resources != nil {
			jobSummary.Resources = deploymentmodels.ConvertRadixResourceRequirements(*radixBatchJob.Resources)
		} else if len(batchRdJobComponent.Resources.Requests) > 0 || len(batchRdJobComponent.Resources.Limits) > 0 {
			jobSummary.Resources = deploymentmodels.ConvertRadixResourceRequirements(batchRdJobComponent.Resources)
		}
	}

	var activeDeployJobComponent *radixv1.RadixDeployJobComponent
	if activeRd != nil {
		if jc, ok := slice.FindFirst(activeRd.Spec.Jobs, predicate.IsRadixDeployJobComponentWithName(radixBatch.Spec.RadixDeploymentJobRef.Job)); ok {
			activeDeployJobComponent = &jc
		}
	}
	jobSchedulerBatchStatus := jobschedulerbatch.GetRadixBatchStatus(radixBatch, activeDeployJobComponent)
	jobSchedulerBatchJobName := fmt.Sprintf("%s-%s", radixBatch.GetName(), radixBatchJob.Name)
	if jobSchedulerBatchJobStatus, ok := slice.FindFirst(jobSchedulerBatchStatus.JobStatuses, func(js jobschedulermodels.Job) bool {
		return js.Name == jobSchedulerBatchJobName
	}); ok {
		jobSummary.Status = mapScheduledBatchJobStatus(jobSchedulerBatchJobStatus.Status)
		jobSummary.Created = jobSchedulerBatchJobStatus.CreationTime
		jobSummary.Started = jobSchedulerBatchJobStatus.Started
		jobSummary.Ended = jobSchedulerBatchJobStatus.Ended
		jobSummary.Message = jobSchedulerBatchJobStatus.Message
		jobSummary.FailedCount = jobSchedulerBatchJobStatus.Failed
		jobSummary.Restart = jobSchedulerBatchJobStatus.Restart
	}

	return jobSummary
}

func mapScheduledBatchJobStatus(status radixv1.RadixBatchJobApiStatus) deploymentmodels.ScheduledBatchJobStatus {
	switch status {
	case radixv1.RadixBatchJobApiStatusRunning:
		return deploymentmodels.ScheduledBatchJobStatusRunning
	case radixv1.RadixBatchJobApiStatusSucceeded:
		return deploymentmodels.ScheduledBatchJobStatusSucceeded
	case radixv1.RadixBatchJobApiStatusFailed:
		return deploymentmodels.ScheduledBatchJobStatusFailed
	case radixv1.RadixBatchJobApiStatusWaiting:
		return deploymentmodels.ScheduledBatchJobStatusWaiting
	case radixv1.RadixBatchJobApiStatusStopping:
		return deploymentmodels.ScheduledBatchJobStatusStopping
	case radixv1.RadixBatchJobApiStatusStopped:
		return deploymentmodels.ScheduledBatchJobStatusStopped
	case radixv1.RadixBatchJobApiStatusActive:
		return deploymentmodels.ScheduledBatchJobStatusActive
	case radixv1.RadixBatchJobApiStatusCompleted:
		return deploymentmodels.ScheduledBatchJobStatusCompleted
	default:
		return ""
	}
}
