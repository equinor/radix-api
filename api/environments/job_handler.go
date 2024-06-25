package environments

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	environmentModels "github.com/equinor/radix-api/api/environments/models"
	"github.com/equinor/radix-api/api/kubequery"
	"github.com/equinor/radix-api/api/models"
	"github.com/equinor/radix-api/api/utils"
	radixhttp "github.com/equinor/radix-common/net/http"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	batchesv1 "github.com/equinor/radix-job-scheduler/api/v1/batches"
	jobsSchedulerModels "github.com/equinor/radix-job-scheduler/models"
	jobSchedulerV1Models "github.com/equinor/radix-job-scheduler/models/v1"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixLabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// GetJobs Get jobs
func (eh EnvironmentHandler) GetJobs(ctx context.Context, appName, envName, jobComponentName string) ([]deploymentModels.ScheduledJobSummary, error) {
	jobs, err := eh.getJobs(ctx, appName, envName, jobComponentName)
	if err != nil {
		return nil, err
	}

	sort.SliceStable(jobs, func(i, j int) bool {
		return utils.IsBefore(&jobs[j], &jobs[i])
	})

	return jobs, nil
}

func (eh EnvironmentHandler) getJobs(ctx context.Context, appName, envName, jobComponentName string) ([]deploymentModels.ScheduledJobSummary, error) {
	radixBatches, err := eh.getRadixBatches(ctx, appName, envName, jobComponentName, kube.RadixBatchTypeJob)
	if err != nil {
		return nil, err
	}
	return eh.getScheduledJobSummaryList(radixBatches), nil
}

// GetJob Gets job by name
func (eh EnvironmentHandler) GetJob(ctx context.Context, appName, envName, jobComponentName, jobName string) (*deploymentModels.ScheduledJobSummary, error) {
	return eh.getJob(ctx, appName, envName, jobComponentName, jobName)
}

// StopJob Stop job by name
func (eh EnvironmentHandler) StopJob(ctx context.Context, appName, envName, jobComponentName, jobName string) error {
	batch, jobId, batchJobName, err := eh.getBatchJob(ctx, appName, envName, jobComponentName, jobName)
	if err != nil {
		return err
	}

	nonStoppableJob := slice.FindAll(batch.Status.JobStatuses, func(js radixv1.RadixBatchJobStatus) bool { return js.Name == batchJobName && !isBatchJobStoppable(js) })
	if len(nonStoppableJob) > 0 {
		return radixhttp.ValidationError(jobName, fmt.Sprintf("invalid job running state=%s", nonStoppableJob[0].Phase))
	}

	batch.Spec.Jobs[jobId].Stop = radixutils.BoolPtr(true)
	_, err = eh.accounts.UserAccount.RadixClient.RadixV1().RadixBatches(batch.GetNamespace()).Update(ctx, batch, metav1.UpdateOptions{})
	return err
}

// RestartJob Start running or stopped job by name
func (eh EnvironmentHandler) RestartJob(ctx context.Context, appName, envName, jobComponentName, jobName string) error {
	batch, jobIdx, _, err := eh.getBatchJob(ctx, appName, envName, jobComponentName, jobName)
	if err != nil {
		return err
	}

	setRestartJobTimeout(batch, jobIdx, radixutils.FormatTimestamp(time.Now()))
	_, err = eh.accounts.UserAccount.RadixClient.RadixV1().RadixBatches(batch.GetNamespace()).Update(ctx, batch, metav1.UpdateOptions{})
	return err
}

// RestartBatch Restart a scheduled or stopped batch
func (eh EnvironmentHandler) RestartBatch(ctx context.Context, appName, envName, jobComponentName, batchName string) error {
	batch, err := eh.getRadixBatch(ctx, appName, envName, jobComponentName, batchName, kube.RadixBatchTypeBatch)
	if err != nil {
		return err
	}

	restartTimestamp := radixutils.FormatTimestamp(time.Now())
	for jobIdx := 0; jobIdx < len(batch.Spec.Jobs); jobIdx++ {
		setRestartJobTimeout(batch, jobIdx, restartTimestamp)
	}
	_, err = eh.accounts.UserAccount.RadixClient.RadixV1().RadixBatches(batch.GetNamespace()).Update(ctx, batch, metav1.UpdateOptions{})
	return err
}

func setRestartJobTimeout(batch *radixv1.RadixBatch, jobIdx int, restartTimestamp string) {
	batch.Spec.Jobs[jobIdx].Stop = nil
	batch.Spec.Jobs[jobIdx].Restart = restartTimestamp
}

// DeleteJob Delete job by name
func (eh EnvironmentHandler) DeleteJob(ctx context.Context, appName, envName, jobComponentName, jobName string) error {
	batchName, _, ok := parseBatchAndJobNameFromScheduledJobName(jobName)
	if !ok {
		return jobNotFoundError(jobName)
	}

	batch, err := eh.getRadixBatch(ctx, appName, envName, jobComponentName, batchName, kube.RadixBatchTypeJob)
	if err != nil {
		return err
	}

	return eh.accounts.UserAccount.RadixClient.RadixV1().RadixBatches(batch.GetNamespace()).Delete(ctx, batch.GetName(), metav1.DeleteOptions{})
}

func (eh EnvironmentHandler) getJob(ctx context.Context, appName, envName, jobComponentName, jobName string) (*deploymentModels.ScheduledJobSummary, error) {
	batchName, batchJobName, ok := parseBatchAndJobNameFromScheduledJobName(jobName)
	if !ok {
		return nil, jobNotFoundError(jobName)
	}

	batch, err := eh.getRadixBatch(ctx, appName, envName, jobComponentName, batchName, "")
	if err != nil {
		return nil, err
	}

	jobs := slice.FindAll(batch.Spec.Jobs, func(job radixv1.RadixBatchJob) bool { return job.Name == batchJobName })
	if len(jobs) == 0 {
		return nil, jobNotFoundError(jobName)
	}

	var jobComponent *radixv1.RadixDeployJobComponent
	deploymentName := batch.Spec.RadixDeploymentJobRef.Name
	jobComponent, err = eh.getRadixJobDeployComponent(ctx, appName, envName, jobComponentName, deploymentName)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}
	jobSummary := eh.getScheduledJobSummary(batch, jobs[0], nil, jobComponent)
	return &jobSummary, nil
}

func (eh EnvironmentHandler) getRadixJobDeployComponent(ctx context.Context, appName, envName, jobComponentName, deploymentName string) (*radixv1.RadixDeployJobComponent, error) {
	namespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
	radixDeployment, err := eh.accounts.UserAccount.RadixClient.RadixV1().RadixDeployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	rdJob, _ := slice.FindFirst(radixDeployment.Spec.Jobs, func(job radixv1.RadixDeployJobComponent) bool { return job.Name == jobComponentName })
	return &rdJob, nil
}

// GetBatches Get batches
func (eh EnvironmentHandler) GetBatches(ctx context.Context, appName, envName, jobComponentName string) ([]deploymentModels.ScheduledBatchSummary, error) {
	summaries, err := eh.getBatches(ctx, appName, envName, jobComponentName)
	if err != nil {
		return nil, err
	}

	sort.SliceStable(summaries, func(i, j int) bool {
		return utils.IsBefore(&summaries[j], &summaries[i])
	})

	return summaries, nil
}

func (eh EnvironmentHandler) getBatches(ctx context.Context, appName, envName, jobComponentName string) ([]deploymentModels.ScheduledBatchSummary, error) {
	radixBatches, err := eh.getRadixBatches(ctx, appName, envName, jobComponentName, kube.RadixBatchTypeBatch)
	if err != nil {
		return nil, err
	}

	jobSchedulerBatchHandler, err := eh.getJobSchedulerBatchHandler(ctx, appName, envName, jobComponentName)
	if err != nil {
		return nil, err
	}
	batchStatuses, err := jobSchedulerBatchHandler.GetBatches(ctx)
	if err != nil {
		return nil, err
	}
	return eh.getScheduledBatchSummaryList(radixBatches, batchStatuses), nil
}

// StopBatch Stop batch by name
func (eh EnvironmentHandler) StopBatch(ctx context.Context, appName, envName, jobComponentName, batchName string) error {
	batch, err := eh.getRadixBatch(ctx, appName, envName, jobComponentName, batchName, kube.RadixBatchTypeBatch)
	if err != nil {
		return err
	}

	if !isBatchStoppable(batch.Status.Condition) {
		return nil
	}

	nonStoppableJobs := slice.FindAll(batch.Status.JobStatuses, func(js radixv1.RadixBatchJobStatus) bool { return !isBatchJobStoppable(js) })
	var didChange bool
	for idx, job := range batch.Spec.Jobs {
		if slice.FindIndex(nonStoppableJobs, func(js radixv1.RadixBatchJobStatus) bool { return js.Name == job.Name }) == -1 {
			batch.Spec.Jobs[idx].Stop = radixutils.BoolPtr(true)
			didChange = true
		}
	}

	if !didChange {
		return nil
	}

	_, err = eh.accounts.UserAccount.RadixClient.RadixV1().RadixBatches(batch.GetNamespace()).Update(ctx, batch, metav1.UpdateOptions{})
	return err
}

// DeleteBatch Delete batch by name
func (eh EnvironmentHandler) DeleteBatch(ctx context.Context, appName, envName, jobComponentName, batchName string) error {
	batch, err := eh.getRadixBatch(ctx, appName, envName, jobComponentName, batchName, kube.RadixBatchTypeBatch)
	if err != nil {
		return err
	}

	return eh.accounts.UserAccount.RadixClient.RadixV1().RadixBatches(batch.GetNamespace()).Delete(ctx, batch.GetName(), metav1.DeleteOptions{})
}

// CopyBatch Copy batch by name
func (eh EnvironmentHandler) CopyBatch(ctx context.Context, appName, envName, jobComponentName, batchName string, scheduledBatchRequest environmentModels.ScheduledBatchRequest) (*deploymentModels.ScheduledBatchSummary, error) {
	deploymentName := scheduledBatchRequest.DeploymentName
	jobSchedulerBatchHandler, err := eh.jobSchedulerBatchHandler(ctx, appName, envName, jobComponentName, deploymentName)
	if err != nil {
		return nil, err
	}
	batchStatus, err := jobSchedulerBatchHandler.CopyBatch(ctx, batchName, deploymentName)
	if err != nil {
		return nil, err
	}
	return eh.getScheduledBatchStatus(batchStatus, deploymentName), nil
}

func (eh EnvironmentHandler) jobSchedulerBatchHandler(ctx context.Context, appName string, envName string, jobComponentName string, deploymentName string) (batchesv1.BatchHandler, error) {
	jobComponent, err := eh.getRadixJobDeployComponent(ctx, appName, envName, jobComponentName, deploymentName)
	if err != nil {
		return nil, err
	}
	jobSchedulerBatchHandler := eh.jobSchedulerHandlerFactory.CreateJobSchedulerBatchHandlerForEnv(getJobSchedulerEnvFor(appName, envName, jobComponentName, deploymentName), jobComponent)
	return jobSchedulerBatchHandler, nil
}

// CopyJob Copy job by name
func (eh EnvironmentHandler) CopyJob(ctx context.Context, appName, envName, jobComponentName, jobName string, scheduledJobRequest environmentModels.ScheduledJobRequest) (*deploymentModels.ScheduledJobSummary, error) {
	deploymentName := scheduledJobRequest.DeploymentName
	jobComponent, err := eh.getRadixJobDeployComponent(ctx, appName, envName, jobComponentName, deploymentName)
	if err != nil {
		return nil, err
	}
	jobSchedulerJobHandler := eh.jobSchedulerHandlerFactory.CreateJobSchedulerJobHandlerForEnv(getJobSchedulerEnvFor(appName, envName, jobComponentName, deploymentName), jobComponent)
	jobStatus, err := jobSchedulerJobHandler.CopyJob(ctx, jobName, deploymentName)
	if err != nil {
		return nil, err
	}
	return eh.getScheduledJobStatus(jobStatus, deploymentName), nil
}

// GetBatch Gets batch by name
func (eh EnvironmentHandler) GetBatch(ctx context.Context, appName, envName, jobComponentName, batchName string) (*deploymentModels.ScheduledBatchSummary, error) {
	return eh.getBatch(ctx, appName, envName, jobComponentName, batchName)
}

func (eh EnvironmentHandler) getBatch(ctx context.Context, appName, envName, jobComponentName, batchName string) (*deploymentModels.ScheduledBatchSummary, error) {
	radixBatch, err := eh.getRadixBatch(ctx, appName, envName, jobComponentName, batchName, kube.RadixBatchTypeBatch)
	if err != nil {
		return nil, err
	}

	jobSchedulerBatchHandler, err := eh.getJobSchedulerBatchHandler(ctx, appName, envName, jobComponentName)
	if err != nil {
		return nil, err
	}
	batchStatus, err := jobSchedulerBatchHandler.GetBatch(ctx, batchName)
	if err != nil {
		return nil, err
	}
	batchSummary := eh.getScheduledBatchSummary(radixBatch, batchStatus)
	batchSummary.JobList = eh.getScheduledJobSummaries(radixBatch, batchStatus.JobStatuses)
	return &batchSummary, nil

}

func (eh EnvironmentHandler) getJobSchedulerBatchHandler(ctx context.Context, appName string, envName string, jobComponentName string) (batchesv1.BatchHandler, error) {
	rdList, err := kubequery.GetRadixDeploymentsForEnvironments(ctx, eh.accounts.UserAccount.RadixClient, appName, []string{envName}, 1)
	if err != nil {
		return nil, err
	}
	activeRd, ok := models.GetActiveDeploymentForAppEnv(appName, envName, rdList)
	if !ok {
		return nil, fmt.Errorf("no active deployment found for the app %s, environment %s", appName, envName)
	}
	jobSchedulerBatchHandler, err := eh.jobSchedulerBatchHandler(ctx, appName, envName, jobComponentName, activeRd.GetName())
	if err != nil {
		return nil, err
	}
	return jobSchedulerBatchHandler, nil
}

// GetJobPayload Gets job payload
func (eh EnvironmentHandler) GetJobPayload(ctx context.Context, appName, envName, jobComponentName, jobName string) (io.ReadCloser, error) {
	return eh.getJobPayload(ctx, appName, envName, jobComponentName, jobName)
}

func (eh EnvironmentHandler) getJobPayload(ctx context.Context, appName, envName, jobComponentName, jobName string) (io.ReadCloser, error) {
	batchName, batchJobName, ok := parseBatchAndJobNameFromScheduledJobName(jobName)
	if !ok {
		return nil, jobNotFoundError(jobName)
	}

	batch, err := eh.getRadixBatch(ctx, appName, envName, jobComponentName, batchName, "")
	if err != nil {
		return nil, err
	}

	jobs := slice.FindAll(batch.Spec.Jobs, func(job radixv1.RadixBatchJob) bool { return job.Name == batchJobName })
	if len(jobs) == 0 {
		return nil, jobNotFoundError(jobName)
	}

	job := jobs[0]
	if job.PayloadSecretRef == nil {
		return io.NopCloser(&bytes.Buffer{}), nil
	}

	namespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
	secret, err := eh.accounts.ServiceAccount.Client.CoreV1().Secrets(namespace).Get(ctx, job.PayloadSecretRef.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, environmentModels.ScheduledJobPayloadNotFoundError(appName, jobName)
		}
		return nil, err
	}

	payload, ok := secret.Data[job.PayloadSecretRef.Key]
	if !ok {
		return nil, environmentModels.ScheduledJobPayloadNotFoundError(appName, jobName)
	}

	return io.NopCloser(bytes.NewReader(payload)), nil
}

func (eh EnvironmentHandler) getRadixBatches(ctx context.Context, appName, envName, jobComponentName string, batchType kube.RadixBatchType) ([]radixv1.RadixBatch, error) {
	namespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
	selector := radixLabels.Merge(
		radixLabels.ForApplicationName(appName),
		radixLabels.ForComponentName(jobComponentName),
		radixLabels.ForBatchType(batchType),
	)

	batches, err := eh.accounts.UserAccount.RadixClient.RadixV1().RadixBatches(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}

	return batches.Items, nil
}

func (eh EnvironmentHandler) getRadixBatch(ctx context.Context, appName, envName, jobComponentName, batchName string, batchType kube.RadixBatchType) (*radixv1.RadixBatch, error) {
	namespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
	labelSelector := radixLabels.Merge(
		radixLabels.ForApplicationName(appName),
		radixLabels.ForComponentName(jobComponentName),
	)

	if batchType != "" {
		labelSelector = radixLabels.Merge(
			labelSelector,
			radixLabels.ForBatchType(batchType),
		)
	}

	batch, err := eh.accounts.UserAccount.RadixClient.RadixV1().RadixBatches(namespace).Get(ctx, batchName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, batchNotFoundError(batchName)
		}
		return nil, err
	}

	if !labelSelector.AsSelector().Matches(labels.Set(batch.GetLabels())) {
		return nil, batchNotFoundError(batchName)
	}

	return batch, nil
}

func (eh EnvironmentHandler) getScheduledBatchSummaryList(batches []radixv1.RadixBatch, batchStatuses []jobSchedulerV1Models.BatchStatus) []deploymentModels.ScheduledBatchSummary {
	batchStatusesMap := slice.Reduce(batchStatuses, make(map[string]*jobSchedulerV1Models.BatchStatus), func(acc map[string]*jobSchedulerV1Models.BatchStatus, batchStatus jobSchedulerV1Models.BatchStatus) map[string]*jobSchedulerV1Models.BatchStatus {
		acc[batchStatus.Name] = &batchStatus
		return acc
	})
	var summaries []deploymentModels.ScheduledBatchSummary
	for _, batch := range batches {
		batchStatus := batchStatusesMap[batch.Name]
		summaries = append(summaries, eh.getScheduledBatchSummary(&batch, batchStatus))
	}
	return summaries
}

func (eh EnvironmentHandler) getScheduledBatchSummary(radixBatch *radixv1.RadixBatch, batchStatus *jobSchedulerV1Models.BatchStatus) deploymentModels.ScheduledBatchSummary {
	var jobStatuses []jobSchedulerV1Models.JobStatus
	if batchStatus != nil {
		jobStatuses = batchStatus.JobStatuses
	}
	summary := deploymentModels.ScheduledBatchSummary{
		Name:           radixBatch.Name,
		TotalJobCount:  len(radixBatch.Spec.Jobs),
		DeploymentName: radixBatch.Spec.RadixDeploymentJobRef.Name,
		JobList:        eh.getScheduledJobSummaries(radixBatch, jobStatuses),
	}
	if batchStatus != nil {
		summary.Status = string(batchStatus.Status)
		summary.Created = batchStatus.Created
		summary.Started = batchStatus.Started
		summary.Ended = batchStatus.Ended
	} else {
		summary.Status = string(radixBatch.Status.Condition.Type)
		summary.Created = radixutils.FormatTimestamp(radixBatch.GetCreationTimestamp().Time)
		summary.Started = radixutils.FormatTime(radixBatch.Status.Condition.ActiveTime)
		summary.Ended = radixutils.FormatTime(radixBatch.Status.Condition.CompletionTime)
	}
	return summary
}

func (eh EnvironmentHandler) getScheduledBatchStatus(batchStatus *jobSchedulerV1Models.BatchStatus, deploymentName string) *deploymentModels.ScheduledBatchSummary {
	return &deploymentModels.ScheduledBatchSummary{
		Name:           batchStatus.Name,
		DeploymentName: deploymentName,
		Status:         string(batchStatus.Status),
		TotalJobCount:  len(batchStatus.JobStatuses),
		Created:        batchStatus.Created,
		Started:        batchStatus.Started,
		Ended:          batchStatus.Ended,
	}
}

func (eh EnvironmentHandler) getScheduledJobStatus(jobStatus *jobSchedulerV1Models.JobStatus, deploymentName string) *deploymentModels.ScheduledJobSummary {
	return &deploymentModels.ScheduledJobSummary{
		Name:           fmt.Sprintf("%s-%s", jobStatus.BatchName, jobStatus.Name),
		DeploymentName: deploymentName,
		BatchName:      jobStatus.BatchName,
		JobId:          jobStatus.JobId,
		Status:         string(jobStatus.Status),
	}
}

func (eh EnvironmentHandler) getScheduledJobSummaryList(batches []radixv1.RadixBatch) []deploymentModels.ScheduledJobSummary {
	var summaries []deploymentModels.ScheduledJobSummary
	for _, batch := range batches {
		summaries = append(summaries, eh.getScheduledJobSummaries(&batch, nil)...)
	}
	return summaries
}

func (eh EnvironmentHandler) getScheduledJobSummaries(radixBatch *radixv1.RadixBatch, jobStatuses []jobSchedulerV1Models.JobStatus) (summaries []deploymentModels.ScheduledJobSummary) {
	jobStatusMap := slice.Reduce(jobStatuses, make(map[string]*jobSchedulerV1Models.JobStatus), func(acc map[string]*jobSchedulerV1Models.JobStatus, jobStatus jobSchedulerV1Models.JobStatus) map[string]*jobSchedulerV1Models.JobStatus {
		acc[jobStatus.Name] = &jobStatus
		return acc
	})
	for _, job := range radixBatch.Spec.Jobs {
		jobName := fmt.Sprintf("%s-%s", radixBatch.Name, job.Name)
		jobStatus, _ := jobStatusMap[jobName]
		summaries = append(summaries, eh.getScheduledJobSummary(radixBatch, job, jobStatus, nil))
	}
	return
}

func (eh EnvironmentHandler) getScheduledJobSummary(radixBatch *radixv1.RadixBatch, job radixv1.RadixBatchJob, jobStatus *jobSchedulerV1Models.JobStatus, jobComponent *radixv1.RadixDeployJobComponent) deploymentModels.ScheduledJobSummary {
	var batchName string
	if radixBatch.GetLabels()[kube.RadixBatchTypeLabel] == string(kube.RadixBatchTypeBatch) {
		batchName = radixBatch.GetName()
	}

	summary := deploymentModels.ScheduledJobSummary{
		Name:           fmt.Sprintf("%s-%s", radixBatch.GetName(), job.Name),
		DeploymentName: radixBatch.Spec.RadixDeploymentJobRef.Name,
		BatchName:      batchName,
		JobId:          job.JobId,
		ReplicaList:    getReplicaSummariesForJob(radixBatch, job),
		Status:         radixv1.RadixBatchJobApiStatusWaiting,
	}

	if jobComponent != nil {
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
			summary.Node = (*deploymentModels.Node)(&jobComponent.Node)
		}
		if job.Node != nil {
			summary.Node = (*deploymentModels.Node)(job.Node)
		}

		if job.Resources != nil {
			summary.Resources = deploymentModels.ConvertRadixResourceRequirements(*job.Resources)
		} else if len(jobComponent.Resources.Requests) > 0 || len(jobComponent.Resources.Limits) > 0 {
			summary.Resources = deploymentModels.ConvertRadixResourceRequirements(jobComponent.Resources)
		}
	}
	if jobStatus != nil {
		summary.Status = string(jobStatus.Status)
		summary.Created = jobStatus.Created
		summary.Started = jobStatus.Started
		summary.Ended = jobStatus.Ended
		summary.Message = jobStatus.Message
		summary.FailedCount = jobStatus.Failed
		summary.Restart = jobStatus.Restart
	}
	return summary
}

func getReplicaSummariesForJob(radixBatch *radixv1.RadixBatch, job radixv1.RadixBatchJob) []deploymentModels.ReplicaSummary {
	if jobStatus, ok := slice.FindFirst(radixBatch.Status.JobStatuses, func(jobStatus radixv1.RadixBatchJobStatus) bool {
		return jobStatus.Name == job.Name
	}); ok {
		return slice.Reduce(jobStatus.RadixBatchJobPodStatuses, make([]deploymentModels.ReplicaSummary, 0),
			func(acc []deploymentModels.ReplicaSummary, status radixv1.RadixBatchJobPodStatus) []deploymentModels.ReplicaSummary {
				return append(acc, getReplicaSummaryByJobPodStatus(status, job))
			})
	}
	return nil
}

func getReplicaSummaryByJobPodStatus(status radixv1.RadixBatchJobPodStatus, job radixv1.RadixBatchJob) deploymentModels.ReplicaSummary {
	summary := deploymentModels.ReplicaSummary{
		Name:          status.Name,
		Created:       radixutils.FormatTimestamp(status.CreationTime.Time),
		RestartCount:  status.RestartCount,
		Image:         status.Image,
		ImageId:       status.ImageID,
		PodIndex:      status.PodIndex,
		Reason:        status.Reason,
		StatusMessage: status.Message,
		ExitCode:      status.ExitCode,
		Status:        getReplicaStatusByJobPodStatusPhase(status.Phase),
	}
	if status.StartTime != nil {
		summary.StartTime = radixutils.FormatTimestamp(status.StartTime.Time)
	}
	if status.EndTime != nil {
		summary.EndTime = radixutils.FormatTimestamp(status.EndTime.Time)
	}
	if job.Resources != nil {
		summary.Resources = pointers.Ptr(deploymentModels.ConvertRadixResourceRequirements(*job.Resources))
	}
	return summary
}

func getReplicaStatusByJobPodStatusPhase(statusPhase radixv1.RadixBatchJobPodPhase) deploymentModels.ReplicaStatus {
	replicaStatus := deploymentModels.ReplicaStatus{}
	switch statusPhase {
	case radixv1.PodFailed:
		replicaStatus.Status = "Failed"
	case radixv1.PodRunning:
		replicaStatus.Status = "Running"
	case radixv1.PodStopped:
		replicaStatus.Status = "Stopped"
	case radixv1.PodSucceeded:
		replicaStatus.Status = "Succeeded"
	default:
		replicaStatus.Status = "Pending"
	}
	return replicaStatus
}

// check if batch can be stopped
func isBatchStoppable(condition radixv1.RadixBatchCondition) bool {
	return condition.Type == "" ||
		condition.Type == radixv1.BatchConditionTypeActive ||
		condition.Type == radixv1.BatchConditionTypeWaiting
}

// check if batch job can be stopped
func isBatchJobStoppable(status radixv1.RadixBatchJobStatus) bool {
	return status.Phase == "" ||
		status.Phase == radixv1.BatchJobPhaseActive ||
		status.Phase == radixv1.BatchJobPhaseWaiting ||
		status.Phase == radixv1.BatchJobPhaseRunning
}

func batchNotFoundError(batchName string) error {
	return radixhttp.NotFoundError(fmt.Sprintf("batch %s not found", batchName))
}

func jobNotFoundError(jobName string) error {
	return radixhttp.NotFoundError(fmt.Sprintf("job %s not found", jobName))
}

func parseBatchAndJobNameFromScheduledJobName(scheduleJobName string) (batchName, batchJobName string, ok bool) {
	scheduleJobNameParts := strings.Split(scheduleJobName, "-")
	if len(scheduleJobNameParts) < 2 {
		return
	}
	batchName = strings.Join(scheduleJobNameParts[:len(scheduleJobNameParts)-1], "-")
	batchJobName = scheduleJobNameParts[len(scheduleJobNameParts)-1]
	ok = true
	return
}

func (eh EnvironmentHandler) getBatchJob(ctx context.Context, appName string, envName string, jobComponentName string, jobName string) (*radixv1.RadixBatch, int, string, error) {
	batchName, batchJobName, ok := parseBatchAndJobNameFromScheduledJobName(jobName)
	if !ok {
		return nil, 0, "", jobNotFoundError(jobName)
	}

	batch, err := eh.getRadixBatch(ctx, appName, envName, jobComponentName, batchName, "")
	if err != nil {
		return nil, 0, "", err
	}

	idx := slice.FindIndex(batch.Spec.Jobs, func(job radixv1.RadixBatchJob) bool { return job.Name == batchJobName })
	if idx == -1 {
		return nil, 0, "", jobNotFoundError(jobName)
	}
	return batch, idx, batchJobName, err
}

func getJobSchedulerEnvFor(appName, envName, jobComponentName, deploymentName string) *jobsSchedulerModels.Env {
	return &jobsSchedulerModels.Env{
		RadixComponentName:                           jobComponentName,
		RadixDeploymentName:                          deploymentName,
		RadixDeploymentNamespace:                     operatorUtils.GetEnvironmentNamespace(appName, envName),
		RadixJobSchedulersPerEnvironmentHistoryLimit: 10,
	}
}
