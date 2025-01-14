package models

import (
	"fmt"
	"time"

	deploymentmodels "github.com/equinor/radix-api/api/deployments/models"
	utils2 "github.com/equinor/radix-api/api/utils"
	"github.com/equinor/radix-api/api/utils/predicate"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	jobschedulermodels "github.com/equinor/radix-job-scheduler/models/v2"
	jobschedulerbatch "github.com/equinor/radix-job-scheduler/pkg/batch"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
)

// GetScheduledBatchSummaryList Get scheduled batch summary list
// func GetScheduledBatchSummaryList(radixBatches []radixv1.RadixBatch, batchStatuses []jobschedulermodels.RadixBatch, radixDeploymentMap map[string]radixv1.RadixDeployment, jobComponentName string) []deploymentmodels.ScheduledBatchSummary {
// 	batchStatusesMap := slice.Reduce(batchStatuses, make(map[string]*jobschedulermodels.RadixBatch), func(acc map[string]*jobschedulermodels.RadixBatch, radixBatchStatus jobschedulermodels.RadixBatch) map[string]*jobschedulermodels.RadixBatch {
// 		acc[radixBatchStatus.Name] = &radixBatchStatus
// 		return acc
// 	})
// 	var summaries []deploymentmodels.ScheduledBatchSummary
// 	for _, radixBatch := range radixBatches {
// 		batchStatus := batchStatusesMap[radixBatch.Name]
// 		radixDeployJobComponent := GetBatchDeployJobComponent(radixBatch.Spec.RadixDeploymentJobRef.Name, jobComponentName, radixDeploymentMap)
// 		summaries = append(summaries, GetScheduledBatchSummary(&radixBatch, batchStatus, radixDeployJobComponent))
// 	}
// 	return summaries
// }

// GetScheduledSingleJobSummaryList Get scheduled single job summary list
// func GetScheduledSingleJobSummaryList(radixBatches []radixv1.RadixBatch, batchStatuses []jobschedulermodels.RadixBatch, radixDeploymentMap map[string]radixv1.RadixDeployment, jobComponentName string) []deploymentmodels.ScheduledJobSummary {
// 	batchStatusesMap := slice.Reduce(batchStatuses, make(map[string]*jobschedulermodels.RadixBatch), func(acc map[string]*jobschedulermodels.RadixBatch, radixBatchStatus jobschedulermodels.RadixBatch) map[string]*jobschedulermodels.RadixBatch {
// 		acc[radixBatchStatus.Name] = &radixBatchStatus
// 		return acc
// 	})
// 	var summaries []deploymentmodels.ScheduledJobSummary
// 	for _, radixBatch := range radixBatches {
// 		radixBatchStatus := batchStatusesMap[radixBatch.Name]
// 		batchDeployJobComponent := GetBatchDeployJobComponent(radixBatch.Spec.RadixDeploymentJobRef.Name, jobComponentName, radixDeploymentMap)
// 		summaries = append(summaries, GetScheduledJobSummary(&radixBatch, &radixBatch.Spec.Jobs[0], radixBatchStatus, batchDeployJobComponent))
// 	}
// 	return summaries
// }

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
func GetScheduledBatchSummary(radixBatch *radixv1.RadixBatch, radixBatchStatus *jobschedulermodels.RadixBatch, radixDeployJobComponent *radixv1.RadixDeployJobComponent) deploymentmodels.ScheduledBatchSummary {
	summary := deploymentmodels.ScheduledBatchSummary{
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
func GetScheduledJobSummaryList(radixBatch *radixv1.RadixBatch, radixBatchStatus *jobschedulermodels.RadixBatch, radixDeployJobComponent *radixv1.RadixDeployJobComponent) []deploymentmodels.ScheduledJobSummary {
	var summaries []deploymentmodels.ScheduledJobSummary
	for _, radixBatchJob := range radixBatch.Spec.Jobs {
		summaries = append(summaries, GetScheduledJobSummary(radixBatch, &radixBatchJob, radixBatchStatus, radixDeployJobComponent))
	}
	return summaries
}

func BuildScheduledBatchSummaryList(radixBatchList []radixv1.RadixBatch, rdList []radixv1.RadixDeployment) []deploymentmodels.ScheduledBatchSummary {
	var activeRd *radixv1.RadixDeployment
	if rd, ok := slice.FindFirst(rdList, predicate.IsActiveRadixDeployment); ok {
		activeRd = &rd
	}

	return slice.Map(radixBatchList, func(rb radixv1.RadixBatch) deploymentmodels.ScheduledBatchSummary {
		var batchRd *radixv1.RadixDeployment
		if rd, ok := slice.FindFirst(rdList, predicate.IsRadixDeploymentForRadixBatch(&rb)); ok {
			batchRd = &rd
		}
		return *BuildScheduledBatchSummary(&rb, batchRd, activeRd)
	})
}

func BuildScheduledBatchSummary(radixBatch *radixv1.RadixBatch, batchRd, activeRd *radixv1.RadixDeployment) *deploymentmodels.ScheduledBatchSummary {
	var activeDeployJobComponent *radixv1.RadixDeployJobComponent
	if activeRd != nil {
		if jc, ok := slice.FindFirst(activeRd.Spec.Jobs, predicate.IsRadixDeployJobComponentWithName(radixBatch.Spec.RadixDeploymentJobRef.Job)); ok {
			activeDeployJobComponent = &jc
		}
	}
	jobSchedulerBatchStatus := jobschedulerbatch.GetRadixBatchStatus(radixBatch, activeDeployJobComponent)

	batchSummary := deploymentmodels.ScheduledBatchSummary{
		Name:           radixBatch.Name,
		BatchId:        radixBatch.Spec.BatchId,
		TotalJobCount:  len(radixBatch.Spec.Jobs),
		DeploymentName: radixBatch.Spec.RadixDeploymentJobRef.Name,
		JobList:        BuildScheduleJobSummaryList(radixBatch, batchRd, activeRd),
		Created:        &jobSchedulerBatchStatus.CreationTime,
		Started:        jobSchedulerBatchStatus.Started,
		Ended:          jobSchedulerBatchStatus.Ended,
		Status:         utils2.GetBatchJobStatusByJobApiStatus(jobSchedulerBatchStatus.Status),
	}

	return &batchSummary
}

func BuildScheduleJobSummaryList(radixBatch *radixv1.RadixBatch, batchRd, activeRd *radixv1.RadixDeployment) []deploymentmodels.ScheduledJobSummary {
	return slice.Map(radixBatch.Spec.Jobs, func(job radixv1.RadixBatchJob) deploymentmodels.ScheduledJobSummary {
		return *BuildScheduleJobSummary(radixBatch, &job, batchRd, activeRd)
	})
}

func BuildScheduleJobSummary(radixBatch *radixv1.RadixBatch, radixBatchJob *radixv1.RadixBatchJob, batchRd, activeRd *radixv1.RadixDeployment) *deploymentmodels.ScheduledJobSummary {
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
	if jobSchedulerBatchJobStatus, ok := slice.FindFirst(jobSchedulerBatchStatus.JobStatuses, func(js jobschedulermodels.RadixBatchJobStatus) bool {
		return js.Name == jobSchedulerBatchJobName
	}); ok {
		jobSummary.Status = utils2.GetBatchJobStatusByJobApiStatus(jobSchedulerBatchJobStatus.Status)
		jobSummary.Created = jobSchedulerBatchJobStatus.CreationTime
		jobSummary.Started = jobSchedulerBatchJobStatus.Started
		jobSummary.Ended = jobSchedulerBatchJobStatus.Ended
		jobSummary.Message = jobSchedulerBatchJobStatus.Message
		jobSummary.FailedCount = jobSchedulerBatchJobStatus.Failed
		jobSummary.Restart = jobSchedulerBatchJobStatus.Restart
	}

	return &jobSummary
}

// GetScheduledJobSummary Get scheduled job summary
func GetScheduledJobSummary(radixBatch *radixv1.RadixBatch, radixBatchJob *radixv1.RadixBatchJob, radixBatchStatus *jobschedulermodels.RadixBatch, radixDeployJobComponent *radixv1.RadixDeployJobComponent) deploymentmodels.ScheduledJobSummary {
	var batchName string
	if radixBatch.GetLabels()[kube.RadixBatchTypeLabel] == string(kube.RadixBatchTypeBatch) {
		batchName = radixBatch.GetName()
	}

	summary := deploymentmodels.ScheduledJobSummary{
		Name:           fmt.Sprintf("%s-%s", radixBatch.GetName(), radixBatchJob.Name),
		DeploymentName: radixBatch.Spec.RadixDeploymentJobRef.Name,
		BatchName:      batchName,
		JobId:          radixBatchJob.JobId,
		Status:         radixv1.RadixBatchJobApiStatusWaiting,
	}

	if jobStatus, ok := slice.FindFirst(radixBatch.Status.JobStatuses, predicate.IsBatchJobStatusForJobName(radixBatchJob.Name)); ok {
		summary.ReplicaList = buildReplicaSummaryListFromBatchJobStatus(jobStatus)
	}

	if radixDeployJobComponent != nil {
		summary.Runtime = &deploymentmodels.Runtime{
			Architecture: operatorutils.GetArchitectureFromRuntime(radixDeployJobComponent.GetRuntime()),
		}
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
			summary.Node = (*deploymentmodels.Node)(&radixDeployJobComponent.Node)
		}
		if radixBatchJob.Node != nil {
			summary.Node = (*deploymentmodels.Node)(radixBatchJob.Node)
		}

		if radixBatchJob.Resources != nil {
			summary.Resources = deploymentmodels.ConvertRadixResourceRequirements(*radixBatchJob.Resources)
		} else if len(radixDeployJobComponent.Resources.Requests) > 0 || len(radixDeployJobComponent.Resources.Limits) > 0 {
			summary.Resources = deploymentmodels.ConvertRadixResourceRequirements(radixDeployJobComponent.Resources)
		}
	}
	if radixBatchStatus == nil {
		return summary
	}
	jobName := fmt.Sprintf("%s-%s", radixBatch.GetName(), radixBatchJob.Name)
	if jobStatus, ok := slice.FindFirst(radixBatchStatus.JobStatuses, func(jobStatus jobschedulermodels.RadixBatchJobStatus) bool {
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
