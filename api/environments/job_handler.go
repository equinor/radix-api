package environments

import (
	"context"
	"fmt"
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	"github.com/equinor/radix-api/api/utils"
	radixhttp "github.com/equinor/radix-common/net/http"
	radixutils "github.com/equinor/radix-common/utils"
	jobSchedulerModels "github.com/equinor/radix-job-scheduler/api/jobs"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
	jobs, jobPodMap, err := getComponentJobsByNamespace(kubeClient, namespace, jobComponentName)
	if err != nil {
		return nil, err
	}
	jobPodMap, err = getJobPodsMap(kubeClient, namespace)
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
	job, err := getJob(eh.kubeUtil.KubeClient(), namespace, jobComponentName, jobName, kube.RadixJobTypeJobSchedule)
	if err != nil {
		return nil, err
	}
	jobPodMap, err := getJobPodsMap(eh.kubeUtil.KubeClient(), namespace)
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
	jobs, _, err := getComponentBatches(eh.kubeUtil.KubeClient(), namespace, jobComponentName)
	if err != nil {
		return nil, err
	}
	jobSummaryList := eh.getScheduledBatchSummaryList(jobs, nil)
	return jobSummaryList, nil
}

// GetBatch Gets batch by name
func (eh EnvironmentHandler) GetBatch(appName, envName, jobComponentName, batchName string) (*deploymentModels.
	ScheduledBatchSummary, error) {
	namespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
	batch, err := getJob(eh.kubeUtil.KubeClient(), namespace, jobComponentName, batchName, kube.RadixJobTypeBatchSchedule)
	if err != nil {
		return nil, err
	}
	//TODO
	//jobPodMap, err := getJobPodsMap(eh.kubeUtil.KubeClient(), namespace)
	//if err != nil {
	//    return nil, err
	//}
	jobSummary := eh.getScheduledBatchSummary(batch, nil)
	return jobSummary, nil
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
	jobStatus := jobSchedulerModels.GetJobStatusFromJob(eh.kubeUtil.KubeClient(), job, jobPodsMap[job.Name])
	summary.Status = jobStatus.Status
	summary.Message = jobStatus.Message
	return &summary
}

func (eh EnvironmentHandler) getScheduledBatchSummaryList(jobs []batchv1.Job,
	batchJobsMap map[string][]corev1.Pod) []deploymentModels.ScheduledBatchSummary {
	var summaries []deploymentModels.ScheduledBatchSummary
	for _, job := range jobs {
		creationTimestamp := job.GetCreationTimestamp()
		summary := deploymentModels.ScheduledBatchSummary{
			Name:    job.Name,
			Created: radixutils.FormatTimestamp(creationTimestamp.Time),
			Started: radixutils.FormatTime(job.Status.StartTime),
			Ended:   radixutils.FormatTime(job.Status.CompletionTime),
		}
		//TODO
		//jobStatus := jobSchedulerModels.GetJobStatusFromJob(eh.kubeUtil.KubeClient(), &job,
		//batchJobsMap[job.Name])
		//summary.Status = jobStatus.Status
		//summary.Message = jobStatus.Message
		summaries = append(summaries, summary)
	}

	// Sort job-summaries descending
	sort.Slice(summaries, func(i, j int) bool {
		return utils.IsBefore(&summaries[j], &summaries[i])
	})
	return summaries
}

func (eh EnvironmentHandler) getScheduledBatchSummary(job *batchv1.Job,
	jobPodsMap map[string][]corev1.Pod) *deploymentModels.ScheduledBatchSummary {
	creationTimestamp := job.GetCreationTimestamp()
	summary := deploymentModels.ScheduledBatchSummary{
		Name:    job.Name,
		Created: radixutils.FormatTimestamp(creationTimestamp.Time),
		Started: radixutils.FormatTime(job.Status.StartTime),
		Ended:   radixutils.FormatTime(job.Status.CompletionTime),
	}
	//TODO
	//if jobPods, ok := jobPodsMap[job.Name]; ok {
	//    summary.ReplicaList = getReplicaSummariesForPods(jobPods)
	//}
	//jobStatus := jobSchedulerModels.GetJobStatusFromJob(eh.kubeUtil.KubeClient(), job, jobPodsMap[job.Name])
	//summary.Status = jobStatus.Status
	//summary.Message = jobStatus.Message
	return &summary
}

func getComponentJobsByNamespace(client kubernetes.Interface, namespace, componentName string) ([]batchv1.Job, map[string][]corev1.Pod, error) {
	jobs, err := getJobs(client, namespace, componentName, kube.RadixJobTypeJobSchedule)
	if err != nil {
		return nil, nil, err
	}
	if len(jobs) == 0 {
		return nil, nil, nil
	}

	jobPodMap, err := getJobPodsMap(client, namespace)
	if err != nil {
		return nil, nil, err
	}
	return jobs, jobPodMap, nil
}

func getJobPodsMap(client kubernetes.Interface, namespace string) (map[string][]corev1.Pod, error) {
	jobPodLabelSelector := labels.Set{
		kube.RadixJobTypeLabel: kube.RadixJobTypeJobSchedule,
	}
	podList, err := client.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(jobPodLabelSelector).String(),
	})
	if err != nil {
		return nil, err
	}

	jobPodMap := make(map[string][]corev1.Pod)
	for _, pod := range podList.Items {
		pod := pod
		if jobName, ok := pod.GetLabels()[k8sJobNameLabel]; ok {
			jobPodList := jobPodMap[jobName]
			jobPodMap[jobName] = append(jobPodList, pod)
		}
	}
	return jobPodMap, nil
}

func getComponentBatches(client kubernetes.Interface, namespace, componentName string) ([]batchv1.Job, map[string][]batchv1.Job, error) {
	jobs, err := getJobs(client, namespace, componentName, kube.RadixJobTypeBatchSchedule)
	if err != nil {
		return nil, nil, err
	}

	jobPodMap := make(map[string][]batchv1.Job)

	if len(jobs) == 0 {
		return nil, nil, nil
	}
	//TODO
	//jobPodLabelSelector := labels.Set{
	//    kube.RadixJobTypeLabel: kube.RadixJobTypeJobSchedule,
	//}
	//podList, err := client.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
	//    LabelSelector: labels.SelectorFromSet(jobPodLabelSelector).String(),
	//})
	//if err != nil {
	//    return nil, nil, err
	//}
	//
	//for _, pod := range podList.Items {
	//    pod := pod
	//    if jobName, labelExist := pod.GetLabels()[k8sJobNameLabel]; labelExist {
	//        jobPodList := jobPodMap[jobName]
	//        jobPodMap[jobName] = append(jobPodList, pod)
	//    }
	//}
	return jobs, jobPodMap, nil
}

func getComponentBatch(client kubernetes.Interface, namespace, componentName,
	batchName string) (*batchv1.Job, []batchv1.Job, error) {
	job, err := getJob(client, namespace, componentName, batchName, kube.RadixJobTypeBatchSchedule)
	if err != nil {
		return nil, nil, err
	}

	//TODO
	//jobPodMap := make(map[string][]batchv1.Job)
	//
	//if len(jobs) == 0 {
	//    return nil, nil, nil
	//}
	//jobPodLabelSelector := labels.Set{
	//    kube.RadixJobTypeLabel: kube.RadixJobTypeJobSchedule,
	//}
	//podList, err := client.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
	//    LabelSelector: labels.SelectorFromSet(jobPodLabelSelector).String(),
	//})
	//if err != nil {
	//    return nil, nil, err
	//}
	//
	//for _, pod := range podList.Items {
	//    pod := pod
	//    if jobName, labelExist := pod.GetLabels()[k8sJobNameLabel]; labelExist {
	//        jobPodList := jobPodMap[jobName]
	//        jobPodMap[jobName] = append(jobPodList, pod)
	//    }
	//}
	return job, nil, nil
}

func getJobs(client kubernetes.Interface, namespace, componentName, jobType string) ([]batchv1.Job, error) {
	jobLabelSelector := map[string]string{
		kube.RadixComponentLabel: componentName,
		kube.RadixJobTypeLabel:   jobType,
	}
	jobList, err := client.BatchV1().Jobs(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(jobLabelSelector).String(),
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
	if !strings.EqualFold(job.Labels[kube.RadixComponentLabel], componentName) ||
		!strings.EqualFold(job.Labels[kube.RadixJobTypeLabel], jobType) {
		return nil, errors.NewNotFound(batchv1.Resource("Job"), name)
	}
	return job, nil
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
