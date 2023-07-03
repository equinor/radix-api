package models

import (
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	"github.com/equinor/radix-common/utils/slice"
	corev1 "k8s.io/api/core/v1"
)

// BuildReplicaSummaryList builds a list of ReplicaSummary models.
func BuildReplicaSummaryList(podList []corev1.Pod) []deploymentModels.ReplicaSummary {
	return slice.Map(podList, BuildReplicaSummary)
}

// BuildReplicaSummary builds a ReplicaSummary model.
func BuildReplicaSummary(pod corev1.Pod) deploymentModels.ReplicaSummary {
	return deploymentModels.GetReplicaSummary(pod)
}
