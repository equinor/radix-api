package environments

import (
	"context"
	"fmt"
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	jobModels "github.com/equinor/radix-api/api/jobs/models"
	"github.com/equinor/radix-api/api/utils"
	radixhttp "github.com/equinor/radix-common/net/http"
	radixutils "github.com/equinor/radix-common/utils"
	batchSchedulerApi "github.com/equinor/radix-job-scheduler/api/batches"
	jobSchedulerApi "github.com/equinor/radix-job-scheduler/api/jobs"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
	"sort"
	"strings"
)

const (
	k8sJobNameLabel = "job-name" // A label that k8s automatically adds to a Pod created by a Job
)

// GetJobs Get jobs
func (eh EnvironmentHandler) GetJobs(appName, envName, jobComponentName string) ([]deploymentModels.
	ScheduledJobSummary, error) {
	namespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
	kubeClient := eh.kubeUtil.KubeClient()
	jobs, err := getSingleJobs(kubeClient, namespace, jobComponentName)
	if err != nil {
		return nil, err
	}
	jobPodLabelSelector := labels.Set{
		kube.RadixJobTypeLabel: kube.RadixJobTypeJobSchedule,
	}
	podList, err := getPodsForSelector(kubeClient, namespace, labels.SelectorFromSet(jobPodLabelSelector))
	if err != nil {
		return nil, err
	}
	jobPodMap, err := getJobPodsMap(podList)
	if err != nil {
		return nil, err
	}
	jobSummaryList := eh.getScheduledJobSummaryList(jobs, jobPodMap)
	return jobSummaryList, nil
}

// GetJob Gets job by name
func (eh EnvironmentHandler) GetJob(appName, envName, jobComponentName, jobName string) (*deploymentModels.
	ScheduledJobSummary, error) {
	namespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
	kubeClient := eh.kubeUtil.KubeClient()
	job, err := getJob(kubeClient, namespace, jobComponentName, jobName, kube.RadixJobTypeJobSchedule)
	if err != nil {
		return nil, err
	}
	jobPodLabelSelector := labels.Set{
		k8sJobNameLabel:        jobName,
		kube.RadixJobTypeLabel: kube.RadixJobTypeJobSchedule,
	}
	podList, err := getPodsForSelector(kubeClient, namespace, labels.SelectorFromSet(jobPodLabelSelector))
	if err != nil {
		return nil, err
	}
	jobPodMap, err := getJobPodsMap(podList)
	if err != nil {
		return nil, err
	}
	jobSummary := eh.getScheduledJobSummary(job, jobPodMap)
	return jobSummary, nil
}

// GetBatches Get batches
func (eh EnvironmentHandler) GetBatches(appName, envName, jobComponentName string) ([]deploymentModels.
	ScheduledBatchSummary, error) {
	namespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
	batches, err := getBatches(eh.kubeUtil.KubeClient(), namespace, jobComponentName)
	if err != nil {
		return nil, err
	}
	return eh.getScheduledBatchSummaryList(batches)
}

// GetBatch Gets batch by name
func (eh EnvironmentHandler) GetBatch(appName, envName, jobComponentName, batchName string) (*deploymentModels.
	ScheduledBatchSummary, error) {
	namespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
	batch, err := getJob(eh.kubeUtil.KubeClient(), namespace, jobComponentName, batchName, kube.RadixJobTypeBatchSchedule)
	if err != nil {
		return nil, err
	}
	summary, err := eh.getScheduledBatchSummary(batch)
	if err != nil {
		return nil, err
	}
	kubeClient := eh.kubeUtil.KubeClient()
	jobPodLabelSelector := labels.Set{
		kube.RadixBatchNameLabel: batchName,
	}
	batchPods, err := getPodsForSelector(kubeClient, namespace, labels.SelectorFromSet(jobPodLabelSelector))
	if err != nil {
		return nil, err
	}
	batchStatus, err := batchSchedulerApi.GetBatchStatusFromJob(kubeClient, batch, batchPods)
	if err != nil {
		return nil, err
	}
	summary.Status = batchStatus.Status
	summary.Message = batchStatus.Message

	jobPodsMap, err := getJobPodsMap(batchPods)
	if err != nil {
		return nil, err
	}
	if batchPod, ok := jobPodsMap[batchName]; ok && len(batchPod) > 0 {
		batchPodSummary := deploymentModels.GetReplicaSummary(batchPod[0])
		summary.Replica = &batchPodSummary
	}
	batchJobSummaryList, err := eh.getBatchJobSummaryList(err, kubeClient, namespace, jobComponentName, batchName, jobPodsMap)
	if err != nil {
		return nil, err
	}
	summary.JobList = batchJobSummaryList
	return summary, nil
}

func (eh EnvironmentHandler) getBatchJobSummaryList(err error, kubeClient kubernetes.Interface, namespace string, jobComponentName string, batchName string, jobPodsMap map[string][]corev1.Pod) ([]deploymentModels.ScheduledJobSummary, error) {
	var summaries []deploymentModels.ScheduledJobSummary
	batchJobs, err := getBatchJobs(kubeClient, namespace, jobComponentName, batchName)
	if err != nil {
		return nil, err
	}
	for _, job := range batchJobs {
		summaries = append(summaries, *eh.getScheduledJobSummary(&job, jobPodsMap))
	}
	return summaries, nil
}

func (eh EnvironmentHandler) getScheduledJobSummaryList(jobs []batchv1.Job,
	jobPodsMap map[string][]corev1.Pod) []deploymentModels.ScheduledJobSummary {
	var summaries []deploymentModels.ScheduledJobSummary
	for _, job := range jobs {
		summary := eh.getScheduledJobSummary(&job, jobPodsMap)
		summaries = append(summaries, *summary)
	}

	// Sort job-summaries descending
	sort.Slice(summaries, func(i, j int) bool {
		return utils.IsBefore(&summaries[j], &summaries[i])
	})
	return summaries
}

func (eh EnvironmentHandler) getScheduledJobSummary(job *batchv1.Job,
	jobPodsMap map[string][]corev1.Pod) *deploymentModels.ScheduledJobSummary {
	creationTimestamp := job.GetCreationTimestamp()
	summary := deploymentModels.ScheduledJobSummary{
		Name:    job.Name,
		Created: radixutils.FormatTimestamp(creationTimestamp.Time),
		Started: radixutils.FormatTime(job.Status.StartTime),
		Ended:   radixutils.FormatTime(job.Status.CompletionTime),
	}
	if jobPods, ok := jobPodsMap[job.Name]; ok {
		summary.ReplicaList = getReplicaSummariesForPods(jobPods)
	}
	jobStatus := jobSchedulerApi.GetJobStatusFromJob(eh.kubeUtil.KubeClient(), job, jobPodsMap[job.Name])
	summary.Status = jobStatus.Status
	summary.Message = jobStatus.Message
	return &summary
}

func (eh EnvironmentHandler) getScheduledBatchSummaryList(batches []batchv1.Job) ([]deploymentModels.ScheduledBatchSummary, error) {
	var summaries []deploymentModels.ScheduledBatchSummary
	for _, batch := range batches {
		summary, err := eh.getScheduledBatchSummary(&batch)
		if err != nil {
			return nil, err
		}
		summary.Status = jobModels.Succeeded.String() //TODO should be real status?
		summaries = append(summaries, *summary)
	}

	// Sort batch-summaries descending
	sort.Slice(summaries, func(i, j int) bool {
		return utils.IsBefore(&summaries[j], &summaries[i])
	})
	return summaries, nil
}

func (eh EnvironmentHandler) getScheduledBatchSummary(batch *batchv1.Job) (*deploymentModels.ScheduledBatchSummary, error) {
	creationTimestamp := batch.GetCreationTimestamp()
	summary := deploymentModels.ScheduledBatchSummary{
		Name:    batch.Name,
		Created: radixutils.FormatTimestamp(creationTimestamp.Time),
		Started: radixutils.FormatTime(batch.Status.StartTime),
		Ended:   radixutils.FormatTime(batch.Status.CompletionTime),
	}
	return &summary, nil
}

func getJobPodsMap(podList []corev1.Pod) (map[string][]corev1.Pod, error) {
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

func getPodsForSelector(client kubernetes.Interface, namespace string, selector labels.Selector) ([]corev1.Pod, error) {
	podList, err := client.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return nil, err
	}
	return podList.Items, err
}

func getSingleJobs(client kubernetes.Interface, namespace, componentName string) ([]batchv1.Job, error) {
	batchNameNotExistsRequirement, err := labels.NewRequirement(kube.RadixBatchNameLabel, selection.DoesNotExist, nil)
	if err != nil {
		return nil, err
	}
	selector := labels.SelectorFromSet(map[string]string{
		kube.RadixComponentLabel: componentName,
		kube.RadixJobTypeLabel:   kube.RadixJobTypeJobSchedule,
	}).Add(*batchNameNotExistsRequirement)
	return getJobsForLabelSelector(client, namespace, selector)
}

func getBatches(client kubernetes.Interface, namespace, componentName string) ([]batchv1.Job, error) {
	jobLabelSelector := map[string]string{
		kube.RadixComponentLabel: componentName,
		kube.RadixJobTypeLabel:   kube.RadixJobTypeBatchSchedule,
	}
	return getJobsForLabelSelector(client, namespace, labels.SelectorFromSet(jobLabelSelector))
}

func getBatchJobs(client kubernetes.Interface, namespace, componentName, batchName string) ([]batchv1.Job, error) {
	labelSelector := map[string]string{
		kube.RadixComponentLabel: componentName,
		kube.RadixJobTypeLabel:   kube.RadixJobTypeJobSchedule,
		kube.RadixBatchNameLabel: batchName,
	}
	return getJobsForLabelSelector(client, namespace, labels.SelectorFromSet(labelSelector))
}

func getJobsForLabelSelector(client kubernetes.Interface, namespace string, labelSelector labels.Selector) ([]batchv1.Job, error) {
	jobList, err := client.BatchV1().Jobs(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("error getting jobs: %w", err)
	}
	return jobList.Items, err
}

func getJob(client kubernetes.Interface, namespace, componentName, name, jobType string) (*batchv1.Job, error) {
	job, err := client.BatchV1().Jobs(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(job.Labels[kube.RadixComponentLabel], componentName) &&
		strings.EqualFold(job.Labels[kube.RadixJobTypeLabel], jobType) {
		return job, nil
	}
	return nil, jobNotFoundError(name)
}

func getReplicaSummariesForPods(jobPods []corev1.Pod) []deploymentModels.ReplicaSummary {
	var replicaSummaries []deploymentModels.ReplicaSummary
	for _, pod := range jobPods {
		replicaSummaries = append(replicaSummaries, deploymentModels.GetReplicaSummary(pod))
	}
	return replicaSummaries
}

func jobNotFoundError(jobName string) error {
	return radixhttp.NotFoundError(fmt.Sprintf("job %s not found", jobName))
}
