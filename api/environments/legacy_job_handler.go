package environments

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	environmentModels "github.com/equinor/radix-api/api/environments/models"
	jobModels "github.com/equinor/radix-api/api/jobs/models"
	"github.com/equinor/radix-api/api/utils"
	apiModels "github.com/equinor/radix-api/models"
	radixutils "github.com/equinor/radix-common/utils"

	// batchSchedulerApi "github.com/equinor/radix-job-scheduler/api/batches"
	// jobSchedulerApi "github.com/equinor/radix-job-scheduler/api/jobs"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

const (
	legacyRadixBatchJobCountAnnotation = "radix.equinor.com/batch-job-count"
	legacyJobPayloadPropertyName       = "payload"
)

type legacyJobHandler struct {
	accounts apiModels.Accounts
}

// GetJobs Get jobs
func (h legacyJobHandler) GetJobs(appName, envName, jobComponentName string) ([]deploymentModels.
	ScheduledJobSummary, error) {
	namespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
	jobs, err := h.getSingleJobs(namespace, jobComponentName)
	if err != nil {
		return nil, err
	}
	jobPodLabelSelector := labels.Set{
		kube.RadixJobTypeLabel: kube.RadixJobTypeJobSchedule,
	}
	podList, err := h.getPodsForSelector(namespace, labels.SelectorFromSet(jobPodLabelSelector))
	if err != nil {
		return nil, err
	}
	jobPodMap, err := h.getJobPodsMap(podList)
	if err != nil {
		return nil, err
	}
	jobSummaryList := h.getScheduledJobSummaryList(jobs, jobPodMap)
	return jobSummaryList, nil
}

func (h legacyJobHandler) GetJob(appName, envName, jobComponentName, jobName string) (*deploymentModels.
	ScheduledJobSummary, error) {
	namespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
	job, err := h.getJob(namespace, jobComponentName, jobName, kube.RadixJobTypeJobSchedule)
	if err != nil {
		return nil, err
	}
	jobPodLabelSelector := labels.Set{
		k8sJobNameLabel:        jobName,
		kube.RadixJobTypeLabel: kube.RadixJobTypeJobSchedule,
	}
	podList, err := h.getPodsForSelector(namespace, labels.SelectorFromSet(jobPodLabelSelector))
	if err != nil {
		return nil, err
	}
	jobPodMap, err := h.getJobPodsMap(podList)
	if err != nil {
		return nil, err
	}
	jobSummary := h.getScheduledJobSummary(job, jobPodMap)
	return jobSummary, nil
}

// GetBatches Get batches
func (h legacyJobHandler) GetBatches(appName, envName, jobComponentName string) ([]deploymentModels.
	ScheduledBatchSummary, error) {
	namespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
	batches, err := h.getBatches(namespace, jobComponentName)
	if err != nil {
		return nil, err
	}
	return h.getScheduledBatchSummaryList(batches)
}

func (h legacyJobHandler) getScheduledJobSummaryList(jobs []batchv1.Job,
	jobPodsMap map[string][]corev1.Pod) []deploymentModels.ScheduledJobSummary {
	summaries := make([]deploymentModels.ScheduledJobSummary, 0) //return an array - not null
	for _, job := range jobs {
		summary := h.getScheduledJobSummary(&job, jobPodsMap)
		summaries = append(summaries, *summary)
	}

	// Sort job-summaries descending
	sort.Slice(summaries, func(i, j int) bool {
		return utils.IsBefore(&summaries[j], &summaries[i])
	})
	return summaries
}

func (h legacyJobHandler) GetBatch(appName, envName, jobComponentName, batchName string) (*deploymentModels.
	ScheduledBatchSummary, error) {
	namespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
	batch, err := h.getJob(namespace, jobComponentName, batchName, kube.RadixJobTypeBatchSchedule)
	if err != nil {
		return nil, err
	}
	summary, err := h.getScheduledBatchSummary(batch)
	if err != nil {
		return nil, err
	}
	kubeClient := h.accounts.UserAccount.Client
	jobPodLabelSelector := labels.Set{
		kube.RadixBatchNameLabel: batchName,
	}
	batchPods, err := h.getPodsForSelector(namespace, labels.SelectorFromSet(jobPodLabelSelector))
	if err != nil {
		return nil, err
	}
	batchStatus, err := GetBatchStatusFromJob(kubeClient, batch, batchPods)
	if err != nil {
		return nil, err
	}
	summary.Status = batchStatus.Status
	//lint:ignore SA1019 support old batch scheduler
	summary.Message = batchStatus.Message

	jobPodsMap, err := h.getJobPodsMap(batchPods)
	if err != nil {
		return nil, err
	}
	if batchPod, ok := jobPodsMap[batchName]; ok && len(batchPod) > 0 {
		batchPodSummary := deploymentModels.GetReplicaSummary(batchPod[0])
		//lint:ignore SA1019 support old batch scheduler
		summary.Replica = &batchPodSummary
	}
	batchJobSummaryList, err := h.getBatchJobSummaryList(namespace, jobComponentName, batchName, jobPodsMap)
	if err != nil {
		return nil, err
	}
	summary.JobList = batchJobSummaryList
	return summary, nil
}

// GetJobPayload Gets job payload
func (h legacyJobHandler) GetJobPayload(appName, envName, jobComponentName, jobName string) (io.ReadCloser, error) {
	namespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
	kubeUtil, err := kube.New(h.accounts.ServiceAccount.Client, h.accounts.UserAccount.RadixClient, nil)
	if err != nil {
		return nil, err
	}
	payloadSecrets, err := kubeUtil.ListSecretsWithSelector(namespace, h.getJobsSchedulerPayloadSecretSelector(appName, jobComponentName, jobName))
	if err != nil {
		return nil, err
	}
	if len(payloadSecrets) == 0 {
		return nil, environmentModels.ScheduledJobPayloadNotFoundError(appName, jobName)
	}
	if len(payloadSecrets) > 1 {
		return nil, environmentModels.ScheduledJobPayloadUnexpectedError(appName, jobName, "unexpected multiple payloads found")
	}
	payload := payloadSecrets[0].Data[legacyJobPayloadPropertyName]
	return io.NopCloser(bytes.NewReader(payload)), nil
}

func (h legacyJobHandler) getJobsSchedulerPayloadSecretSelector(appName, jobComponentName, jobName string) string {
	return labels.SelectorFromSet(map[string]string{
		kube.RadixAppLabel:       appName,
		kube.RadixComponentLabel: jobComponentName,
		kube.RadixJobTypeLabel:   kube.RadixJobTypeJobSchedule,
		kube.RadixJobNameLabel:   jobName,
	}).String()
}

func (h legacyJobHandler) getScheduledJobSummary(job *batchv1.Job,
	jobPodsMap map[string][]corev1.Pod) *deploymentModels.ScheduledJobSummary {
	creationTimestamp := job.GetCreationTimestamp()
	batchName := job.ObjectMeta.Labels[kube.RadixBatchNameLabel]
	summary := deploymentModels.ScheduledJobSummary{
		Name:      job.Name,
		Created:   radixutils.FormatTimestamp(creationTimestamp.Time),
		Started:   radixutils.FormatTime(job.Status.StartTime),
		BatchName: batchName,
		JobId:     "", // TODO: was job.ObjectMeta.Labels[kube.RadixJobIdLabel],
	}
	summary.TimeLimitSeconds = job.Spec.Template.Spec.ActiveDeadlineSeconds
	jobPods := jobPodsMap[job.Name]
	if len(jobPods) > 0 {
		summary.ReplicaList = getReplicaSummariesForPods(jobPods)
	}
	summary.Resources = h.getJobResourceRequirements(job, jobPods)
	summary.BackoffLimit = h.getJobBackoffLimit(job)
	jobStatus := GetJobStatusFromJob(h.accounts.UserAccount.Client, job, jobPodsMap[job.Name])
	summary.Status = jobStatus.Status
	summary.Message = jobStatus.Message
	summary.Ended = jobStatus.Ended
	return &summary
}

func (h legacyJobHandler) getScheduledBatchSummaryList(batches []batchv1.Job) ([]deploymentModels.ScheduledBatchSummary, error) {
	summaries := make([]deploymentModels.ScheduledBatchSummary, 0) //return an array - not null
	for _, batch := range batches {
		summary, err := h.getScheduledBatchSummary(&batch)
		if err != nil {
			return nil, err
		}
		summary.Status = jobModels.Succeeded.String() //TODO should be real status?
		summaries = append(summaries, *summary)
	}

	return summaries, nil
}

func (h legacyJobHandler) getScheduledBatchSummary(batch *batchv1.Job) (*deploymentModels.ScheduledBatchSummary, error) {
	creationTimestamp := batch.GetCreationTimestamp()
	summary := deploymentModels.ScheduledBatchSummary{
		Name:    batch.Name,
		Created: radixutils.FormatTimestamp(creationTimestamp.Time),
		Started: radixutils.FormatTime(batch.Status.StartTime),
		Ended:   radixutils.FormatTime(batch.Status.CompletionTime),
	}
	if jobCount, ok := batch.ObjectMeta.Annotations[legacyRadixBatchJobCountAnnotation]; ok {
		if count, err := strconv.Atoi(jobCount); err == nil {
			summary.TotalJobCount = count
		} else {
			log.Errorf("failed to get job count for the annotation %s",
				legacyRadixBatchJobCountAnnotation)
		}
	}

	return &summary, nil
}

func (h legacyJobHandler) getBatchJobSummaryList(namespace string, jobComponentName string, batchName string, jobPodsMap map[string][]corev1.Pod) ([]deploymentModels.ScheduledJobSummary, error) {
	summaries := make([]deploymentModels.ScheduledJobSummary, 0) //return an array - not null
	batchJobs, err := h.getBatchJobs(namespace, jobComponentName, batchName)
	if err != nil {
		return nil, err
	}
	for _, job := range batchJobs {
		summaries = append(summaries, *h.getScheduledJobSummary(&job, jobPodsMap))
	}
	return summaries, nil
}

func (h legacyJobHandler) getJobBackoffLimit(job *batchv1.Job) int32 {
	if job.Spec.BackoffLimit == nil {
		return 0
	}
	return *job.Spec.BackoffLimit
}

func (h legacyJobHandler) getJobResourceRequirements(job *batchv1.Job, jobPods []corev1.Pod) deploymentModels.ResourceRequirements {
	if len(jobPods) > 0 && len(jobPods[0].Spec.Containers) > 0 {
		return deploymentModels.ConvertResourceRequirements(jobPods[0].Spec.Containers[0].Resources)
	} else if len(job.Spec.Template.Spec.Containers) > 0 {
		return deploymentModels.ConvertResourceRequirements(job.Spec.Template.Spec.Containers[0].Resources)
	}
	return deploymentModels.ResourceRequirements{}
}

func (h legacyJobHandler) getJobPodsMap(podList []corev1.Pod) (map[string][]corev1.Pod, error) {
	jobPodMap := make(map[string][]corev1.Pod)
	for _, pod := range podList {
		pod := pod
		if jobName, ok := pod.GetLabels()[k8sJobNameLabel]; ok {
			jobPodList := jobPodMap[jobName]
			jobPodMap[jobName] = append(jobPodList, pod)
		}
	}
	return jobPodMap, nil
}

func (h legacyJobHandler) getPodsForSelector(namespace string, selector labels.Selector) ([]corev1.Pod, error) {
	podList, err := h.accounts.UserAccount.Client.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return nil, err
	}
	return podList.Items, err
}

func (h legacyJobHandler) getSingleJobs(namespace, componentName string) ([]batchv1.Job, error) {
	batchNameNotExistsRequirement, err := labels.NewRequirement(kube.RadixBatchNameLabel, selection.DoesNotExist, nil)
	if err != nil {
		return nil, err
	}
	selector := labels.SelectorFromSet(map[string]string{
		kube.RadixComponentLabel: componentName,
		kube.RadixJobTypeLabel:   kube.RadixJobTypeJobSchedule,
	}).Add(*batchNameNotExistsRequirement)
	return h.getJobsForLabelSelector(namespace, selector)
}

func (h legacyJobHandler) getBatches(namespace, componentName string) ([]batchv1.Job, error) {
	jobLabelSelector := map[string]string{
		kube.RadixComponentLabel: componentName,
		kube.RadixJobTypeLabel:   kube.RadixJobTypeBatchSchedule,
	}
	return h.getJobsForLabelSelector(namespace, labels.SelectorFromSet(jobLabelSelector))
}

func (h legacyJobHandler) getBatchJobs(namespace, componentName, batchName string) ([]batchv1.Job, error) {
	labelSelector := map[string]string{
		kube.RadixComponentLabel: componentName,
		kube.RadixJobTypeLabel:   kube.RadixJobTypeJobSchedule,
		kube.RadixBatchNameLabel: batchName,
	}
	return h.getJobsForLabelSelector(namespace, labels.SelectorFromSet(labelSelector))
}

func (h legacyJobHandler) getJobsForLabelSelector(namespace string, labelSelector labels.Selector) ([]batchv1.Job, error) {
	jobList, err := h.accounts.UserAccount.Client.BatchV1().Jobs(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("error getting jobs: %w", err)
	}
	return jobList.Items, err
}

func (h legacyJobHandler) getJob(namespace, componentName, name, jobType string) (*batchv1.Job, error) {
	job, err := h.accounts.UserAccount.Client.BatchV1().Jobs(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(job.Labels[kube.RadixComponentLabel], componentName) &&
		strings.EqualFold(job.Labels[kube.RadixJobTypeLabel], jobType) {
		return job, nil
	}
	return nil, jobNotFoundError(name)
}
