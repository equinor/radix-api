package models

import (
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	"github.com/equinor/radix-api/api/utils/predicate"
	"github.com/equinor/radix-common/utils/slice"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
)

func getHpaSummary(appName, componentName string, hpaList []autoscalingv2.HorizontalPodAutoscaler) *deploymentModels.HorizontalScalingSummary {
	hpa, ok := slice.FindFirst(hpaList, predicate.IsHpaForComponent(appName, componentName))
	if !ok {
		return nil
	}

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
	// TODO: FIX

	return nil, nil
	// currentResourceUtil := getHpaCurrentMetric(hpa, resourceName)
	//
	// // find resource utilization target
	// var targetResourceUtil *int32
	// targetResourceMetric := operatorutils.GetHpaMetric(hpa, resourceName)
	// if targetResourceMetric != nil {
	// 	targetResourceUtil = targetResourceMetric.Resource.Target.AverageUtilization
	// }
	// return currentResourceUtil, targetResourceUtil
}

// func getHpaCurrentMetric(hpa *autoscalingv2.HorizontalPodAutoscaler, resourceName corev1.ResourceName) *int32 {
// 	for _, metric := range hpa.Status.CurrentMetrics {
// 		if metric.Resource != nil && metric.Resource.Name == resourceName {
// 			return metric.Resource.Current.AverageUtilization
// 		}
// 	}
// 	return nil
// }

// The original function
// func getHpaMetrics(cpuTarget *int32, memoryTarget *int32) []autoscalingv2.MetricSpec {
// 	var metrics []autoscalingv2.MetricSpec
// 	if cpuTarget != nil {
// 		metrics = []autoscalingv2.MetricSpec{
// 			{
// 				Type: autoscalingv2.ResourceMetricSourceType,
// 				Resource: &autoscalingv2.ResourceMetricSource{
// 					Name: corev1.ResourceCPU,
// 					Target: autoscalingv2.MetricTarget{
// 						Type:               autoscalingv2.UtilizationMetricType,
// 						AverageUtilization: cpuTarget,
// 					},
// 				},
// 			},
// 		}
// 	}
//
// 	if memoryTarget != nil {
// 		metrics = append(metrics, autoscalingv2.MetricSpec{
// 			Type: autoscalingv2.ResourceMetricSourceType,
// 			Resource: &autoscalingv2.ResourceMetricSource{
// 				Name: corev1.ResourceMemory,
// 				Target: autoscalingv2.MetricTarget{
// 					Type:               autoscalingv2.UtilizationMetricType,
// 					AverageUtilization: memoryTarget,
// 				},
// 			},
// 		})
// 	}
// 	return metrics
// }
