package models

import (
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	"github.com/equinor/radix-api/api/utils/predicate"
	"github.com/equinor/radix-common/utils/slice"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
)

func getHpaSummary(component radixv1.RadixCommonDeployComponent, hpaList []autoscalingv2.HorizontalPodAutoscaler) *deploymentModels.HorizontalScalingSummary {
	i := slice.FindIndex(hpaList, predicate.IsHpaForComponent(component.GetName()))
	if i == -1 {
		return nil
	}
	hpa := hpaList[i]

	minReplicas := int32(1)
	if hpa.Spec.MinReplicas != nil {
		minReplicas = *hpa.Spec.MinReplicas
	}
	maxReplicas := hpa.Spec.MaxReplicas

	currentCpuUtil, targetCpuUtil := getHpaMetrics(&hpa, corev1.ResourceCPU)
	currentMemoryUtil, targetMemoryUtil := getHpaMetrics(&hpa, corev1.ResourceMemory)

	hpaSummary := deploymentModels.HorizontalScalingSummary{
		MinReplicas:                        minReplicas,
		MaxReplicas:                        maxReplicas,
		CurrentCPUUtilizationPercentage:    currentCpuUtil,
		TargetCPUUtilizationPercentage:     targetCpuUtil,
		CurrentMemoryUtilizationPercentage: currentMemoryUtil,
		TargetMemoryUtilizationPercentage:  targetMemoryUtil,
	}
	return &hpaSummary
}

func getHpaMetrics(hpa *autoscalingv2.HorizontalPodAutoscaler, resourceName corev1.ResourceName) (*int32, *int32) {
	currentResourceUtil := getHpaCurrentMetric(hpa, resourceName)

	// find resource utilization target
	var targetResourceUtil *int32
	targetResourceMetric := operatorutils.GetHpaMetric(hpa, resourceName)
	if targetResourceMetric != nil {
		targetResourceUtil = targetResourceMetric.Resource.Target.AverageUtilization
	}
	return currentResourceUtil, targetResourceUtil
}

func getHpaCurrentMetric(hpa *autoscalingv2.HorizontalPodAutoscaler, resourceName corev1.ResourceName) *int32 {
	for _, metric := range hpa.Status.CurrentMetrics {
		if metric.Resource != nil && metric.Resource.Name == resourceName {
			return metric.Resource.Current.AverageUtilization
		}
	}
	return nil
}
