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

const (
	k8sJobNameLabel = "job-name" // A label that k8s automatically adds to a Pod created by a Job
)

// GetJobs Get jobs
func (eh EnvironmentHandler) GetJobs(appName, envName, jobComponentName string) ([]deploymentModels.ScheduledJobSummary, error) {
	radixBatches, err := eh.getRadixBatches(appName, envName, jobComponentName, kube.RadixBatchTypeJob)
	if err != nil {
		return nil, err
	}

	pods, err := eh.getPodsForJobComponent(appName, envName, jobComponentName)
	if err != nil {
		return nil, err
	}

	jobs := eh.getScheduledJobSummaryList(radixBatches, pods)

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

// GetJob Gets job by name
func (eh EnvironmentHandler) GetJob(appName, envName, jobComponentName, jobName string) (*deploymentModels.ScheduledJobSummary, error) {
	if jobSummary, err := eh.getJob(appName, envName, jobComponentName, jobName); err == nil {
		return jobSummary, nil
	}

	// Backward compatibility: Get job not handled by RadixBatch
	// TODO: Remove when there are no legacy jobs left
	jh := legacyJobHandler{accounts: eh.accounts}
	return jh.GetJob(appName, envName, jobComponentName, jobName)
}

func (eh EnvironmentHandler) getJob(appName, envName, jobComponentName, jobName string) (*deploymentModels.ScheduledJobSummary, error) {
	batchName, batchJobName, ok := parseBatchAndJobNameFromScheduleJobName(jobName)
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

	jobSummary := eh.getScheduledJobSummary(batch, jobs[0], pods, nil)
	return &jobSummary, nil
}

// GetBatches Get batches
func (eh EnvironmentHandler) GetBatches(appName, envName, jobComponentName string) ([]deploymentModels.ScheduledBatchSummary, error) {
	radixBatches, err := eh.getRadixBatches(appName, envName, jobComponentName, kube.RadixBatchTypeBatch)
	if err != nil {
		return nil, err
	}
	summaries := eh.getScheduledBatchSummaryList(radixBatches)

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

// GetBatch Gets batch by name
func (eh EnvironmentHandler) GetBatch(appName, envName, jobComponentName, batchName string) (*deploymentModels.ScheduledBatchSummary, error) {
	if batchSummary, err := eh.getBatch(appName, envName, jobComponentName, batchName); err == nil {
		return batchSummary, nil
	}

	// Backward compatibility: Get batch not handled by RadixBatch
	// TODO: Remove when there are no legacy jobs left
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
	batchName, batchJobName, ok := parseBatchAndJobNameFromScheduleJobName(jobName)
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

func (eh EnvironmentHandler) getRadixBatch(appName, envName, jobComponentName, batchName string, batchType kube.RadixBatchType) (radixv1.RadixBatch, error) {
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

	fieldSelector := labels.Set{"metadata.name": batchName}

	batches, err := eh.accounts.UserAccount.RadixClient.RadixV1().RadixBatches(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector.String(), FieldSelector: fieldSelector.String()})
	if err != nil {
		return radixv1.RadixBatch{}, err
	}

	if len(batches.Items) == 0 {
		return radixv1.RadixBatch{}, batchNotFoundError(batchName)
	}

	return batches.Items[0], nil
}

func (eh EnvironmentHandler) getPodsForJobComponent(appName, envName, jobComponentName string) ([]corev1.Pod, error) {
	namespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
	selector := radixLabels.ForComponentName(jobComponentName)

	return eh.getPodsWithLabelSelector(namespace, selector.String())
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
		summaries = append(summaries, eh.getScheduledBatchSummary(batch))
	}

	return
}

func (eh EnvironmentHandler) getScheduledBatchSummary(batch radixv1.RadixBatch) deploymentModels.ScheduledBatchSummary {
	return deploymentModels.ScheduledBatchSummary{
		Name:          batch.Name,
		Status:        getScheduledBatchStatus(batch).String(),
		TotalJobCount: len(batch.Spec.Jobs),
		Created:       radixutils.FormatTimestamp(batch.GetCreationTimestamp().Time),
		Started:       radixutils.FormatTime(batch.Status.Condition.ActiveTime),
		Ended:         radixutils.FormatTime(batch.Status.Condition.CompletionTime),
	}
}

func (eh EnvironmentHandler) getScheduledJobSummaryList(batches []radixv1.RadixBatch, pods []corev1.Pod) (summaries []deploymentModels.ScheduledJobSummary) {
	for _, batch := range batches {
		summaries = append(summaries, eh.getScheduledJobSummaries(batch, pods)...)
	}

	return
}

func (eh EnvironmentHandler) getScheduledJobSummaries(batch radixv1.RadixBatch, pods []corev1.Pod) (summaries []deploymentModels.ScheduledJobSummary) {
	for _, job := range batch.Spec.Jobs {
		summaries = append(summaries, eh.getScheduledJobSummary(batch, job, pods, nil))
	}

	return
}

func (eh EnvironmentHandler) getScheduledJobSummary(batch radixv1.RadixBatch, job radixv1.RadixBatchJob, pods []corev1.Pod, jobComponent *radixv1.RadixDeployJobComponent) deploymentModels.ScheduledJobSummary {
	var batchName string
	if batch.GetLabels()[kube.RadixBatchTypeLabel] == string(kube.RadixBatchTypeBatch) {
		batchName = batch.GetName()
	}
	jobPods := slice.FindAll(pods, func(pod corev1.Pod) bool {
		return isPodForBatchJob(&pod, batch.Spec.RadixDeploymentJobRef.Job, batch.GetName(), job.Name)
	})

	summary := deploymentModels.ScheduledJobSummary{
		Name:        fmt.Sprintf("%s-%s", batch.GetName(), job.Name),
		BatchName:   batchName,
		JobId:       job.JobId,
		ReplicaList: getReplicaSummariesForPods(jobPods),
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

func getScheduledBatchStatus(batch radixv1.RadixBatch) (status jobSchedulerModels.ProgressStatus) {
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

func batchNotFoundError(batchName string) error {
	return radixhttp.NotFoundError(fmt.Sprintf("batch %s not found", batchName))
}

func jobNotFoundError(jobName string) error {
	return radixhttp.NotFoundError(fmt.Sprintf("job %s not found", jobName))
}

func parseBatchAndJobNameFromScheduleJobName(scheduleJobName string) (batchName, batchJobName string, ok bool) {
	scheduleJobNameParts := strings.Split(scheduleJobName, "-")
	if len(scheduleJobNameParts) < 2 {
		return
	}
	batchName = strings.Join(scheduleJobNameParts[:len(scheduleJobNameParts)-1], "-")
	batchJobName = scheduleJobNameParts[len(scheduleJobNameParts)-1]
	ok = true
	return
}
