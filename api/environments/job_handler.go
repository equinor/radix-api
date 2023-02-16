package environments

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	environmentModels "github.com/equinor/radix-api/api/environments/models"
	"github.com/equinor/radix-api/api/utils"
	radixhttp "github.com/equinor/radix-common/net/http"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	jobSchedulerModels "github.com/equinor/radix-job-scheduler/models"
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
func (eh EnvironmentHandler) GetJobs(appName, envName, jobComponentName string) ([]deploymentModels.ScheduledJobSummary, error) {
	jobs, err := eh.getJobs(appName, envName, jobComponentName)
	if err != nil {
		return nil, err
	}

	// Backward compatibility: Get list of jobs not handled by RadixBatch
	// TODO: Remove when there are no legacy jobs left
	jh := legacyJobHandler{accounts: eh.accounts}
	legacyJobs, err := jh.GetJobs(appName, envName, jobComponentName)
	if err != nil {
		return nil, err
	}
	jobs = append(jobs, legacyJobs...)

	sort.SliceStable(jobs, func(i, j int) bool {
		return utils.IsBefore(&jobs[j], &jobs[i])
	})

	return jobs, nil
}

func (eh EnvironmentHandler) getJobs(appName, envName, jobComponentName string) ([]deploymentModels.ScheduledJobSummary, error) {
	radixBatches, err := eh.getRadixBatches(appName, envName, jobComponentName, kube.RadixBatchTypeJob)
	if err != nil {
		return nil, err
	}

	return eh.getScheduledJobSummaryList(radixBatches, nil), nil
}

// GetJob Gets job by name
func (eh EnvironmentHandler) GetJob(appName, envName, jobComponentName, jobName string) (*deploymentModels.ScheduledJobSummary, error) {
	if jobSummary, err := eh.getJob(appName, envName, jobComponentName, jobName); err == nil {
		return jobSummary, nil
	}

	// TODO: Return error from getJob when legacy handler is removed
	// TODO: Remove when there are no legacy jobs left

	// Backward compatibility: Get job not handled by RadixBatch
	jh := legacyJobHandler{accounts: eh.accounts}
	return jh.GetJob(appName, envName, jobComponentName, jobName)
}

// StopJob Stop job by name
func (eh EnvironmentHandler) StopJob(appName, envName, jobComponentName, jobName string) error {
	batchName, batchJobName, ok := parseBatchAndJobNameFromScheduledJobName(jobName)
	if !ok {
		return jobNotFoundError(jobName)
	}

	batch, err := eh.getRadixBatch(appName, envName, jobComponentName, batchName, "")
	if err != nil {
		return err
	}

	idx := slice.FindIndex(batch.Spec.Jobs, func(job radixv1.RadixBatchJob) bool { return job.Name == batchJobName })
	if idx == -1 {
		return jobNotFoundError(jobName)
	}

	nonStoppableJob := slice.FindAll(batch.Status.JobStatuses, func(js radixv1.RadixBatchJobStatus) bool { return js.Name == batchJobName && !isBatchJobStoppable(js) })
	if len(nonStoppableJob) > 0 {
		return radixhttp.ValidationError(jobName, fmt.Sprintf("invalid job running state=%s", nonStoppableJob[0].Phase))
	}

	batch.Spec.Jobs[idx].Stop = radixutils.BoolPtr(true)
	_, err = eh.accounts.UserAccount.RadixClient.RadixV1().RadixBatches(batch.GetNamespace()).Update(context.TODO(), batch, metav1.UpdateOptions{})
	return err
}

func (eh EnvironmentHandler) getJob(appName, envName, jobComponentName, jobName string) (*deploymentModels.ScheduledJobSummary, error) {
	batchName, batchJobName, ok := parseBatchAndJobNameFromScheduledJobName(jobName)
	if !ok {
		return nil, jobNotFoundError(jobName)
	}

	batch, err := eh.getRadixBatch(appName, envName, jobComponentName, batchName, "")
	if err != nil {
		return nil, err
	}

	jobs := slice.FindAll(batch.Spec.Jobs, func(job radixv1.RadixBatchJob) bool { return job.Name == batchJobName })
	if len(jobs) == 0 {
		return nil, jobNotFoundError(jobName)
	}

	pods, err := eh.getPodsForBatchJob(appName, envName, batchName, batchJobName)
	if err != nil {
		return nil, err
	}

	var jobComponent *radixv1.RadixDeployJobComponent
	namespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
	if rd, err := eh.accounts.UserAccount.RadixClient.RadixV1().RadixDeployments(namespace).Get(context.TODO(), batch.Spec.RadixDeploymentJobRef.Name, metav1.GetOptions{}); err == nil {
		if rdJobs := slice.FindAll(rd.Spec.Jobs, func(job radixv1.RadixDeployJobComponent) bool { return job.Name == jobComponentName }); len(rdJobs) > 0 {
			jobComponent = &rdJobs[0]
		}
	}

	jobSummary := eh.getScheduledJobSummary(batch, jobs[0], pods, jobComponent)
	return &jobSummary, nil

}

// GetBatches Get batches
func (eh EnvironmentHandler) GetBatches(appName, envName, jobComponentName string) ([]deploymentModels.ScheduledBatchSummary, error) {
	summaries, err := eh.getBatches(appName, envName, jobComponentName)
	if err != nil {
		return nil, err
	}

	// Backward compatibility: Get list of batches not handled by RadixBatch
	// TODO: Remove when there are no legacy jobs left
	jh := legacyJobHandler{accounts: eh.accounts}
	legacyBatches, err := jh.GetBatches(appName, envName, jobComponentName)
	if err != nil {
		return nil, err
	}
	summaries = append(summaries, legacyBatches...)

	sort.SliceStable(summaries, func(i, j int) bool {
		return utils.IsBefore(&summaries[j], &summaries[i])
	})

	return summaries, nil
}

func (eh EnvironmentHandler) getBatches(appName, envName, jobComponentName string) ([]deploymentModels.ScheduledBatchSummary, error) {
	radixBatches, err := eh.getRadixBatches(appName, envName, jobComponentName, kube.RadixBatchTypeBatch)
	if err != nil {
		return nil, err
	}

	return eh.getScheduledBatchSummaryList(radixBatches), nil
}

// StopBatch Stop batch by name
func (eh EnvironmentHandler) StopBatch(appName, envName, jobComponentName, batchName string) error {
	batch, err := eh.getRadixBatch(appName, envName, jobComponentName, batchName, kube.RadixBatchTypeBatch)
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

	_, err = eh.accounts.UserAccount.RadixClient.RadixV1().RadixBatches(batch.GetNamespace()).Update(context.TODO(), batch, metav1.UpdateOptions{})
	return err
}

// GetBatch Gets batch by name
func (eh EnvironmentHandler) GetBatch(appName, envName, jobComponentName, batchName string) (*deploymentModels.ScheduledBatchSummary, error) {
	if batchSummary, err := eh.getBatch(appName, envName, jobComponentName, batchName); err == nil {
		return batchSummary, nil
	}

	// TODO: Return error from getBatch when legacy handler is removed
	// TODO: Remove legacy handler when there are no legacy jobs left

	// Backward compatibility: Get batch not handled by RadixBatch
	jh := legacyJobHandler{accounts: eh.accounts}
	return jh.GetBatch(appName, envName, jobComponentName, batchName)
}

func (eh EnvironmentHandler) getBatch(appName, envName, jobComponentName, batchName string) (*deploymentModels.ScheduledBatchSummary, error) {
	batch, err := eh.getRadixBatch(appName, envName, jobComponentName, batchName, kube.RadixBatchTypeBatch)
	if err != nil {
		return nil, err
	}

	batchSummary := eh.getScheduledBatchSummary(batch)
	pods, err := eh.getPodsForBatch(appName, envName, batchName)
	if err != nil {
		return nil, err
	}
	batchSummary.JobList = eh.getScheduledJobSummaries(batch, pods)
	return &batchSummary, nil

}

// GetJobPayload Gets job payload
func (eh EnvironmentHandler) GetJobPayload(appName, envName, jobComponentName, jobName string) (io.ReadCloser, error) {
	if payload, err := eh.getJobPayload(appName, envName, jobComponentName, jobName); err == nil {
		return payload, nil
	}

	// Backward compatibility: Get batch not handled by RadixBatch
	// TODO: Remove when there are no legacy jobs left
	jh := legacyJobHandler{accounts: eh.accounts}
	return jh.GetJobPayload(appName, envName, jobComponentName, jobName)
}

func (eh EnvironmentHandler) getJobPayload(appName, envName, jobComponentName, jobName string) (io.ReadCloser, error) {
	batchName, batchJobName, ok := parseBatchAndJobNameFromScheduledJobName(jobName)
	if !ok {
		return nil, jobNotFoundError(jobName)
	}

	batch, err := eh.getRadixBatch(appName, envName, jobComponentName, batchName, "")
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
	secret, err := eh.accounts.ServiceAccount.Client.CoreV1().Secrets(namespace).Get(context.TODO(), job.PayloadSecretRef.Name, metav1.GetOptions{})
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

func (eh EnvironmentHandler) getRadixBatches(appName, envName, jobComponentName string, batchType kube.RadixBatchType) ([]radixv1.RadixBatch, error) {
	namespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
	selector := radixLabels.Merge(
		radixLabels.ForApplicationName(appName),
		radixLabels.ForComponentName(jobComponentName),
		radixLabels.ForBatchType(batchType),
	)

	batches, err := eh.accounts.UserAccount.RadixClient.RadixV1().RadixBatches(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}

	return batches.Items, nil
}

func (eh EnvironmentHandler) getRadixBatch(appName, envName, jobComponentName, batchName string, batchType kube.RadixBatchType) (*radixv1.RadixBatch, error) {
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

	batch, err := eh.accounts.UserAccount.RadixClient.RadixV1().RadixBatches(namespace).Get(context.TODO(), batchName, metav1.GetOptions{})
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

func (eh EnvironmentHandler) getPodsForBatch(appName, envName, batchName string) ([]corev1.Pod, error) {
	namespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
	selector := radixLabels.ForBatchName(batchName)

	return eh.getPodsWithLabelSelector(namespace, selector.String())
}

func (eh EnvironmentHandler) getPodsForBatchJob(appName, envName, batchName, jobName string) ([]corev1.Pod, error) {
	namespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
	selector := radixLabels.Merge(
		radixLabels.ForBatchName(batchName),
		radixLabels.ForBatchJobName(jobName),
	)

	return eh.getPodsWithLabelSelector(namespace, selector.String())
}

func (eh EnvironmentHandler) getPodsWithLabelSelector(namespace, labelSelector string) ([]corev1.Pod, error) {
	pods, err := eh.accounts.UserAccount.Client.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
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

func (eh EnvironmentHandler) getScheduledJobSummaryList(batches []radixv1.RadixBatch, pods []corev1.Pod) (summaries []deploymentModels.ScheduledJobSummary) {
	for _, batch := range batches {
		summaries = append(summaries, eh.getScheduledJobSummaries(&batch, pods)...)
	}

	return
}

func (eh EnvironmentHandler) getScheduledJobSummaries(batch *radixv1.RadixBatch, pods []corev1.Pod) (summaries []deploymentModels.ScheduledJobSummary) {
	for _, job := range batch.Spec.Jobs {
		summaries = append(summaries, eh.getScheduledJobSummary(batch, job, pods, nil))
	}

	return
}

func (eh EnvironmentHandler) getScheduledJobSummary(batch *radixv1.RadixBatch, job radixv1.RadixBatchJob, pods []corev1.Pod, jobComponent *radixv1.RadixDeployJobComponent) deploymentModels.ScheduledJobSummary {
	var batchName string
	if batch.GetLabels()[kube.RadixBatchTypeLabel] == string(kube.RadixBatchTypeBatch) {
		batchName = batch.GetName()
	}
	jobPods := slice.FindAll(pods, func(pod corev1.Pod) bool {
		return isPodForBatchJob(&pod, batch.Spec.RadixDeploymentJobRef.Job, batch.GetName(), job.Name)
	})

	summary := deploymentModels.ScheduledJobSummary{
		Name:           fmt.Sprintf("%s-%s", batch.GetName(), job.Name),
		DeploymentName: batch.Spec.RadixDeploymentJobRef.Name,
		BatchName:      batchName,
		JobId:          job.JobId,
		ReplicaList:    getReplicaSummariesForPods(jobPods),
		Status:         getScheduledJobStatus(job, "").String(),
	}

	if jobComponent != nil {
		summary.TimeLimitSeconds = jobComponent.TimeLimitSeconds
		if job.TimeLimitSeconds != nil {
			summary.TimeLimitSeconds = job.TimeLimitSeconds
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

	if statuses := slice.FindAll(batch.Status.JobStatuses, func(jobStatus radixv1.RadixBatchJobStatus) bool { return jobStatus.Name == job.Name }); len(statuses) == 1 {
		status := statuses[0]
		summary.Status = getScheduledJobStatus(job, status.Phase).String()
		summary.Created = radixutils.FormatTime(status.CreationTime)
		summary.Started = radixutils.FormatTime(status.StartTime)
		summary.Ended = radixutils.FormatTime(status.EndTime)
		summary.Message = status.Message
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
		status = jobSchedulerModels.Running
	case radixv1.BatchJobPhaseSucceeded:
		status = jobSchedulerModels.Succeeded
	case radixv1.BatchJobPhaseFailed:
		status = jobSchedulerModels.Failed
	case radixv1.BatchJobPhaseStopped:
		status = jobSchedulerModels.Stopped
	}

	var stop bool
	if job.Stop != nil {
		stop = *job.Stop
	}

	if stop && (status == jobSchedulerModels.Waiting || status == jobSchedulerModels.Running) {
		status = jobSchedulerModels.Stopping
	}

	return
}

func getReplicaSummariesForPods(jobPods []corev1.Pod) []deploymentModels.ReplicaSummary {
	var replicaSummaries []deploymentModels.ReplicaSummary
	for _, pod := range jobPods {
		replicaSummaries = append(replicaSummaries, deploymentModels.GetReplicaSummary(pod))
	}
	return replicaSummaries
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
