package models

import (
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	"github.com/equinor/radix-common/utils/slice"
	corev1 "k8s.io/api/core/v1"
)

func BuildReplicaSummaryList(podList []corev1.Pod) []deploymentModels.ReplicaSummary {
	return slice.Map(podList, BuildReplicaSummary)
}

func BuildReplicaSummary(pod corev1.Pod) deploymentModels.ReplicaSummary {
	return deploymentModels.GetReplicaSummary(pod)
}
