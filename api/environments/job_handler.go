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
	"github.com/equinor/radix-api/api/utils"
	radixhttp "github.com/equinor/radix-common/net/http"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	jobsSchedulerModels "github.com/equinor/radix-job-scheduler/models"
	jobSchedulerModels "github.com/equinor/radix-job-scheduler/models/common"
	jobSchedulerV1Models "github.com/equinor/radix-job-scheduler/models/v1"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixLabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	corev1 "k8s.io/api/core/v1"
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
	namespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
	if rd, err := eh.accounts.UserAccount.RadixClient.RadixV1().RadixDeployments(namespace).Get(ctx, batch.Spec.RadixDeploymentJobRef.Name, metav1.GetOptions{}); err == nil {
		if rdJobs := slice.FindAll(rd.Spec.Jobs, func(job radixv1.RadixDeployJobComponent) bool { return job.Name == jobComponentName }); len(rdJobs) > 0 {
			jobComponent = &rdJobs[0]
		}
	}

	jobSummary := eh.getScheduledJobSummary(batch, jobs[0], jobComponent)
	return &jobSummary, nil
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

	return eh.getScheduledBatchSummaryList(radixBatches), nil
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
	jobSchedulerBatchHandler := eh.jobSchedulerHandlerFactory.CreateJobSchedulerBatchHandlerForEnv(getJobSchedulerEnvFor(appName, envName, jobComponentName, deploymentName))
	batchStatus, err := jobSchedulerBatchHandler.CopyBatch(ctx, batchName, deploymentName)
	if err != nil {
		return nil, err
	}
	return eh.getScheduledBatchStatus(batchStatus, deploymentName), nil
}

// CopyJob Copy job by name
func (eh EnvironmentHandler) CopyJob(ctx context.Context, appName, envName, jobComponentName, jobName string, scheduledJobRequest environmentModels.ScheduledJobRequest) (*deploymentModels.ScheduledJobSummary, error) {
	deploymentName := scheduledJobRequest.DeploymentName
	jobSchedulerJobHandler := eh.jobSchedulerHandlerFactory.CreateJobSchedulerJobHandlerForEnv(getJobSchedulerEnvFor(appName, envName, jobComponentName, deploymentName))
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
	batch, err := eh.getRadixBatch(ctx, appName, envName, jobComponentName, batchName, kube.RadixBatchTypeBatch)
	if err != nil {
		return nil, err
	}

	batchSummary := eh.getScheduledBatchSummary(batch)
	batchSummary.JobList = eh.getScheduledJobSummaries(batch)
	return &batchSummary, nil

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

func (eh EnvironmentHandler) getPodsForBatch(ctx context.Context, appName, envName, batchName string) ([]corev1.Pod, error) {
	namespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
	selector := radixLabels.ForBatchName(batchName)

	return eh.getPodsWithLabelSelector(ctx, namespace, selector.String())
}

func (eh EnvironmentHandler) getPodsForBatchJob(ctx context.Context, appName, envName, batchName, jobName string) ([]corev1.Pod, error) {
	namespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
	selector := radixLabels.Merge(
		radixLabels.ForBatchName(batchName),
		radixLabels.ForBatchJobName(jobName),
	)

	return eh.getPodsWithLabelSelector(ctx, namespace, selector.String())
}

func (eh EnvironmentHandler) getPodsWithLabelSelector(ctx context.Context, namespace, labelSelector string) ([]corev1.Pod, error) {
	pods, err := eh.accounts.UserAccount.Client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, err
	}

	return pods.Items, nil
}

func (eh EnvironmentHandler) getScheduledBatchSummaryList(batches []radixv1.RadixBatch) (summaries []deploymentModels.ScheduledBatchSummary) {
	for _, batch := range batches {
		summaries = append(summaries, eh.getScheduledBatchSummary(&batch))
	}

	return
}

func (eh EnvironmentHandler) getScheduledBatchSummary(batch *radixv1.RadixBatch) deploymentModels.ScheduledBatchSummary {
	return deploymentModels.ScheduledBatchSummary{
		Name:           batch.Name,
		DeploymentName: batch.Spec.RadixDeploymentJobRef.Name,
		Status:         getScheduledBatchStatus(batch).String(),
		TotalJobCount:  len(batch.Spec.Jobs),
		Created:        radixutils.FormatTimestamp(batch.GetCreationTimestamp().Time),
		Started:        radixutils.FormatTime(batch.Status.Condition.ActiveTime),
		Ended:          radixutils.FormatTime(batch.Status.Condition.CompletionTime),
	}
}

func (eh EnvironmentHandler) getScheduledBatchStatus(batchStatus *jobSchedulerV1Models.BatchStatus, deploymentName string) *deploymentModels.ScheduledBatchSummary {
	return &deploymentModels.ScheduledBatchSummary{
		Name:           batchStatus.Name,
		DeploymentName: deploymentName,
		Status:         batchStatus.Status,
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
		Status:         jobStatus.Status,
	}
}

func (eh EnvironmentHandler) getScheduledJobSummaryList(batches []radixv1.RadixBatch) (summaries []deploymentModels.ScheduledJobSummary) {
	for _, batch := range batches {
		summaries = append(summaries, eh.getScheduledJobSummaries(&batch)...)
	}

	return
}

func (eh EnvironmentHandler) getScheduledJobSummaries(batch *radixv1.RadixBatch) (summaries []deploymentModels.ScheduledJobSummary) {
	for _, job := range batch.Spec.Jobs {
		summaries = append(summaries, eh.getScheduledJobSummary(batch, job, nil))
	}
	return
}

func (eh EnvironmentHandler) getScheduledJobSummary(batch *radixv1.RadixBatch, job radixv1.RadixBatchJob, jobComponent *radixv1.RadixDeployJobComponent) deploymentModels.ScheduledJobSummary {
	var batchName string
	if batch.GetLabels()[kube.RadixBatchTypeLabel] == string(kube.RadixBatchTypeBatch) {
		batchName = batch.GetName()
	}

	summary := deploymentModels.ScheduledJobSummary{
		Name:           fmt.Sprintf("%s-%s", batch.GetName(), job.Name),
		DeploymentName: batch.Spec.RadixDeploymentJobRef.Name,
		BatchName:      batchName,
		JobId:          job.JobId,
		ReplicaList:    getReplicaSummariesForJob(batch, job),
		Status:         getScheduledJobStatus(job, radixv1.BatchJobPhaseWaiting).String(),
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

	if status, ok := slice.FindFirst(batch.Status.JobStatuses, func(jobStatus radixv1.RadixBatchJobStatus) bool { return jobStatus.Name == job.Name }); ok {
		summary.Status = getScheduledJobStatus(job, status.Phase).String()
		summary.Created = radixutils.FormatTime(status.CreationTime)
		summary.Started = radixutils.FormatTime(status.StartTime)
		summary.Ended = radixutils.FormatTime(status.EndTime)
		summary.Message = status.Message
		summary.FailedCount = status.Failed
		summary.Restart = status.Restart
	}

	return summary
}

func isPodForBatchJob(pod *corev1.Pod, jobComponentName, batchName, batchJobName string) bool {
	return labels.
		SelectorFromSet(
			radixLabels.Merge(
				radixLabels.ForComponentName(jobComponentName),
				radixLabels.ForBatchName(batchName),
				radixLabels.ForBatchJobName(batchJobName),
			)).
		Matches(labels.Set(pod.GetLabels()))
}

func getScheduledBatchStatus(batch *radixv1.RadixBatch) (status jobSchedulerModels.ProgressStatus) {
	status = jobSchedulerModels.Waiting
	switch {
	case batch.Status.Condition.Type == radixv1.BatchConditionTypeActive:
		status = jobSchedulerModels.Running
	case batch.Status.Condition.Type == radixv1.BatchConditionTypeCompleted:
		status = jobSchedulerModels.Succeeded
		if slice.Any(batch.Status.JobStatuses, func(jobStatus radixv1.RadixBatchJobStatus) bool {
			return jobStatus.Phase == radixv1.BatchJobPhaseFailed
		}) {
			status = jobSchedulerModels.Failed
		}
	}
	return
}

func getScheduledJobStatus(job radixv1.RadixBatchJob, phase radixv1.RadixBatchJobPhase) (status jobSchedulerModels.ProgressStatus) {
	status = jobSchedulerModels.Waiting

	switch phase {
	case radixv1.BatchJobPhaseActive:
		status = jobSchedulerModels.Active
	case radixv1.BatchJobPhaseRunning:
		status = jobSchedulerModels.Running
	case radixv1.BatchJobPhaseSucceeded:
		status = jobSchedulerModels.Succeeded
	case radixv1.BatchJobPhaseFailed:
		status = jobSchedulerModels.Failed
	case radixv1.BatchJobPhaseStopped:
		status = jobSchedulerModels.Stopped
	case radixv1.BatchJobPhaseWaiting:
		status = jobSchedulerModels.Waiting
	}

	var stop bool
	if job.Stop != nil {
		stop = *job.Stop
	}
	if stop && (status == jobSchedulerModels.Waiting || status == jobSchedulerModels.Active || status == jobSchedulerModels.Running) {
		return jobSchedulerModels.Stopping
	}
	return status
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
		replicaStatus.Status = "Failing"
	case radixv1.PodRunning:
		replicaStatus.Status = "Running"
	case radixv1.PodSucceeded:
		replicaStatus.Status = "Terminated"
	default:
		replicaStatus.Status = "Pending"
	}
	return replicaStatus
}

// check if batch can be stopped
func isBatchStoppable(condition radixv1.RadixBatchCondition) bool {
	return condition.Type == "" || condition.Type == radixv1.BatchConditionTypeActive || condition.Type == radixv1.BatchConditionTypeWaiting
}

// check if batch job can be stopped
func isBatchJobStoppable(status radixv1.RadixBatchJobStatus) bool {
	return status.Phase == "" || status.Phase == radixv1.BatchJobPhaseActive || status.Phase == radixv1.BatchJobPhaseWaiting
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
