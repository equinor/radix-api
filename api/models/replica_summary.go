package models

import (
	deploymentmodels "github.com/equinor/radix-api/api/deployments/models"
	"github.com/equinor/radix-common/utils/slice"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
)

// buildReplicaSummaryListFromPods builds a list of ReplicaSummary models from list of pods.
func buildReplicaSummaryListFromPods(podList []corev1.Pod, lastEventWarnings map[string]string) []deploymentmodels.ReplicaSummary {
	return slice.Map(podList, func(pod corev1.Pod) deploymentmodels.ReplicaSummary {
		return deploymentmodels.GetReplicaSummary(pod, lastEventWarnings[pod.GetName()])
	})
}

// buildReplicaSummaryFromBatchJobPodStatus Get replica summary from batch job pod status
func buildReplicaSummaryFromBatchJobPodStatus(jobPodStatus radixv1.RadixBatchJobPodStatus) deploymentmodels.ReplicaSummary {
	statusFunc := func() deploymentmodels.ContainerStatus {
		switch jobPodStatus.Phase {
		case radixv1.PodPending:
			return deploymentmodels.Pending
		case radixv1.PodRunning:
			return deploymentmodels.Running
		case radixv1.PodFailed:
			return deploymentmodels.Failed
		case radixv1.PodStopped:
			return deploymentmodels.Stopped
		case radixv1.PodSucceeded:
			return deploymentmodels.Succeeded
		default:
			return ""
		}
	}

	summary := deploymentmodels.ReplicaSummary{
		Name:          jobPodStatus.Name,
		Created:       jobPodStatus.CreationTime.Time,
		RestartCount:  jobPodStatus.RestartCount,
		Image:         jobPodStatus.Image,
		ImageId:       jobPodStatus.ImageID,
		PodIndex:      jobPodStatus.PodIndex,
		Reason:        jobPodStatus.Reason,
		StatusMessage: jobPodStatus.Message,
		ExitCode:      jobPodStatus.ExitCode,
		Status:        deploymentmodels.ReplicaStatus{Status: statusFunc()},
	}
	if jobPodStatus.StartTime != nil {
		summary.ContainerStarted = &jobPodStatus.StartTime.Time
	}
	if jobPodStatus.EndTime != nil {
		summary.EndTime = &jobPodStatus.EndTime.Time
	}
	return summary
}

func buildReplicaSummaryListFromBatchJobStatus(jobStatus radixv1.RadixBatchJobStatus) []deploymentmodels.ReplicaSummary {
	return slice.Map(jobStatus.RadixBatchJobPodStatuses, buildReplicaSummaryFromBatchJobPodStatus)
}
