package models

import (
	"fmt"
	"time"

	"github.com/equinor/radix-api/api/deployments/models"
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	utils2 "github.com/equinor/radix-api/api/utils"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	jobSchedulerModels "github.com/equinor/radix-job-scheduler/models/v2"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// GetScheduledBatchSummaryList Get scheduled batch summary list
func GetScheduledBatchSummaryList(radixBatches []*radixv1.RadixBatch, batchStatuses []jobSchedulerModels.RadixBatch, radixDeploymentMap map[string]radixv1.RadixDeployment, jobComponentName string) []models.ScheduledBatchSummary {
	batchStatusesMap := slice.Reduce(batchStatuses, make(map[string]*jobSchedulerModels.RadixBatch), func(acc map[string]*jobSchedulerModels.RadixBatch, radixBatchStatus jobSchedulerModels.RadixBatch) map[string]*jobSchedulerModels.RadixBatch {
		acc[radixBatchStatus.Name] = &radixBatchStatus
		return acc
	})
	var summaries []models.ScheduledBatchSummary
	for _, radixBatch := range radixBatches {
		batchStatus := batchStatusesMap[radixBatch.Name]
		radixDeployJobComponent := GetBatchDeployJobComponent(radixBatch.Spec.RadixDeploymentJobRef.Name, jobComponentName, radixDeploymentMap)
		summaries = append(summaries, GetScheduledBatchSummary(radixBatch, batchStatus, radixDeployJobComponent))
	}
	return summaries
}

// GetScheduledSingleJobSummaryList Get scheduled single job summary list
func GetScheduledSingleJobSummaryList(radixBatches []*radixv1.RadixBatch, batchStatuses []jobSchedulerModels.RadixBatch, radixDeploymentMap map[string]radixv1.RadixDeployment, jobComponentName string) []models.ScheduledJobSummary {
	batchStatusesMap := slice.Reduce(batchStatuses, make(map[string]*jobSchedulerModels.RadixBatch), func(acc map[string]*jobSchedulerModels.RadixBatch, radixBatchStatus jobSchedulerModels.RadixBatch) map[string]*jobSchedulerModels.RadixBatch {
		acc[radixBatchStatus.Name] = &radixBatchStatus
		return acc
	})
	var summaries []models.ScheduledJobSummary
	for _, radixBatch := range radixBatches {
		radixBatchStatus := batchStatusesMap[radixBatch.Name]
		batchDeployJobComponent := GetBatchDeployJobComponent(radixBatch.Spec.RadixDeploymentJobRef.Name, jobComponentName, radixDeploymentMap)
		summaries = append(summaries, GetScheduledJobSummary(radixBatch, &radixBatch.Spec.Jobs[0], radixBatchStatus, batchDeployJobComponent))
	}
	return summaries
}

// GetBatchDeployJobComponent Get batch deploy job component
func GetBatchDeployJobComponent(radixDeploymentName string, jobComponentName string, radixDeploymentsMap map[string]radixv1.RadixDeployment) *radixv1.RadixDeployJobComponent {
	if radixDeployment, ok := radixDeploymentsMap[radixDeploymentName]; ok {
		if jobComponent, ok := slice.FindFirst(radixDeployment.Spec.Jobs, func(component radixv1.RadixDeployJobComponent) bool {
			return component.Name == jobComponentName
		}); ok {
			return &jobComponent
		}
	}
	return nil
}

// GetScheduledBatchSummary Get scheduled batch summary
func GetScheduledBatchSummary(radixBatch *radixv1.RadixBatch, radixBatchStatus *jobSchedulerModels.RadixBatch, radixDeployJobComponent *radixv1.RadixDeployJobComponent) models.ScheduledBatchSummary {
	summary := models.ScheduledBatchSummary{
		Name:           radixBatch.Name,
		BatchId:        radixBatch.Spec.BatchId,
		TotalJobCount:  len(radixBatch.Spec.Jobs),
		DeploymentName: radixBatch.Spec.RadixDeploymentJobRef.Name,
		JobList:        GetScheduledJobSummaryList(radixBatch, radixBatchStatus, radixDeployJobComponent),
	}
	if radixBatchStatus != nil {
		summary.Status = utils2.GetBatchJobStatusByJobApiStatus(radixBatchStatus.Status)
		summary.Created = &radixBatchStatus.CreationTime
		summary.Started = radixBatchStatus.Started
		summary.Ended = radixBatchStatus.Ended
	} else {
		var started, ended *time.Time
		if radixBatch.Status.Condition.ActiveTime != nil {
			started = &radixBatch.Status.Condition.ActiveTime.Time
		}
		if radixBatch.Status.Condition.CompletionTime != nil {
			ended = &radixBatch.Status.Condition.CompletionTime.Time
		}
		summary.Status = utils2.GetBatchJobStatusByJobApiCondition(radixBatch.Status.Condition.Type)
		summary.Created = pointers.Ptr(radixBatch.GetCreationTimestamp().Time)
		summary.Started = started
		summary.Ended = ended
	}
	return summary
}

// GetScheduledJobSummaryList Get scheduled job summaries
func GetScheduledJobSummaryList(radixBatch *radixv1.RadixBatch, radixBatchStatus *jobSchedulerModels.RadixBatch, radixDeployJobComponent *radixv1.RadixDeployJobComponent) []models.ScheduledJobSummary {
	var summaries []models.ScheduledJobSummary
	for _, radixBatchJob := range radixBatch.Spec.Jobs {
		summaries = append(summaries, GetScheduledJobSummary(radixBatch, &radixBatchJob, radixBatchStatus, radixDeployJobComponent))
	}
	return summaries
}

// GetScheduledJobSummary Get scheduled job summary
func GetScheduledJobSummary(radixBatch *radixv1.RadixBatch, radixBatchJob *radixv1.RadixBatchJob, radixBatchStatus *jobSchedulerModels.RadixBatch, radixDeployJobComponent *radixv1.RadixDeployJobComponent) models.ScheduledJobSummary {
	var batchName string
	if radixBatch.GetLabels()[kube.RadixBatchTypeLabel] == string(kube.RadixBatchTypeBatch) {
		batchName = radixBatch.GetName()
	}

	summary := models.ScheduledJobSummary{
		Name:           fmt.Sprintf("%s-%s", radixBatch.GetName(), radixBatchJob.Name),
		DeploymentName: radixBatch.Spec.RadixDeploymentJobRef.Name,
		BatchName:      batchName,
		JobId:          radixBatchJob.JobId,
		ReplicaList:    getReplicaSummaryListForJob(radixBatch, *radixBatchJob),
		Status:         radixv1.RadixBatchJobApiStatusWaiting,
		Runtime:        deploymentModels.NewRuntime(radixBatchJob.Runtime),
	}

	if radixDeployJobComponent != nil {
		summary.TimeLimitSeconds = radixDeployJobComponent.TimeLimitSeconds
		if radixBatchJob.TimeLimitSeconds != nil {
			summary.TimeLimitSeconds = radixBatchJob.TimeLimitSeconds
		}
		if radixDeployJobComponent.BackoffLimit != nil {
			summary.BackoffLimit = *radixDeployJobComponent.BackoffLimit
		}
		if radixBatchJob.BackoffLimit != nil {
			summary.BackoffLimit = *radixBatchJob.BackoffLimit
		}

		if radixDeployJobComponent.Node != (radixv1.RadixNode{}) {
			summary.Node = (*models.Node)(&radixDeployJobComponent.Node)
		}
		if radixBatchJob.Node != nil {
			summary.Node = (*models.Node)(radixBatchJob.Node)
		}

		if radixBatchJob.Resources != nil {
			summary.Resources = models.ConvertRadixResourceRequirements(*radixBatchJob.Resources)
		} else if len(radixDeployJobComponent.Resources.Requests) > 0 || len(radixDeployJobComponent.Resources.Limits) > 0 {
			summary.Resources = models.ConvertRadixResourceRequirements(radixDeployJobComponent.Resources)
		}
	}
	if radixBatchStatus == nil {
		return summary
	}
	jobName := fmt.Sprintf("%s-%s", radixBatch.GetName(), radixBatchJob.Name)
	if jobStatus, ok := slice.FindFirst(radixBatchStatus.JobStatuses, func(jobStatus jobSchedulerModels.RadixBatchJobStatus) bool {
		return jobStatus.Name == jobName
	}); ok {
		summary.Status = utils2.GetBatchJobStatusByJobApiStatus(jobStatus.Status)
		summary.Created = jobStatus.CreationTime
		summary.Started = jobStatus.Started
		summary.Ended = jobStatus.Ended
		summary.Message = jobStatus.Message
		summary.FailedCount = jobStatus.Failed
		summary.Restart = jobStatus.Restart
	}
	return summary
}

// GetReplicaSummaryByJobPodStatus Get replica summary by job pod status
func GetReplicaSummaryByJobPodStatus(radixBatchJob radixv1.RadixBatchJob, jobPodStatus radixv1.RadixBatchJobPodStatus) models.ReplicaSummary {
	summary := models.ReplicaSummary{
		Name:          jobPodStatus.Name,
		Created:       jobPodStatus.CreationTime.Time,
		RestartCount:  jobPodStatus.RestartCount,
		Image:         jobPodStatus.Image,
		ImageId:       jobPodStatus.ImageID,
		PodIndex:      jobPodStatus.PodIndex,
		Reason:        jobPodStatus.Reason,
		StatusMessage: jobPodStatus.Message,
		ExitCode:      jobPodStatus.ExitCode,
		Status:        models.ReplicaStatus{Status: getReplicaStatusByPodStatus(jobPodStatus.Phase)},
	}
	if jobPodStatus.StartTime != nil {
		summary.ContainerStarted = &jobPodStatus.StartTime.Time
	}
	if jobPodStatus.EndTime != nil {
		summary.EndTime = &jobPodStatus.EndTime.Time
	}
	if radixBatchJob.Resources != nil {
		summary.Resources = pointers.Ptr(models.ConvertRadixResourceRequirements(*radixBatchJob.Resources))
	}
	return summary
}

func getReplicaSummaryListForJob(radixBatch *radixv1.RadixBatch, radixBatchJob radixv1.RadixBatchJob) []models.ReplicaSummary {
	if jobStatus, ok := slice.FindFirst(radixBatch.Status.JobStatuses, func(jobStatus radixv1.RadixBatchJobStatus) bool {
		return jobStatus.Name == radixBatchJob.Name
	}); ok {
		return slice.Reduce(jobStatus.RadixBatchJobPodStatuses, make([]models.ReplicaSummary, 0),
			func(acc []models.ReplicaSummary, jobPodStatus radixv1.RadixBatchJobPodStatus) []models.ReplicaSummary {
				return append(acc, GetReplicaSummaryByJobPodStatus(radixBatchJob, jobPodStatus))
			})
	}
	return nil
}

func getReplicaStatusByPodStatus(podPhase radixv1.RadixBatchJobPodPhase) deploymentModels.ContainerStatus {
	switch podPhase {
	case radixv1.PodPending:
		return deploymentModels.Pending
	case radixv1.PodRunning:
		return deploymentModels.Running
	case radixv1.PodFailed:
		return deploymentModels.Failed
	case radixv1.PodStopped:
		return deploymentModels.Stopped
	case radixv1.PodSucceeded:
		return deploymentModels.Succeeded
	default:
		return ""
	}
}
