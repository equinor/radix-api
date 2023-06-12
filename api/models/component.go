package models

import (
	"strings"

	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	"github.com/equinor/radix-api/api/utils/predicate"
	commonutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	operatorapplicationconfig "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	operatordefaults "github.com/equinor/radix-operator/pkg/apis/defaults"
	operatordeployment "github.com/equinor/radix-operator/pkg/apis/deployment"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
)

func BuildComponents(ra *radixv1.RadixApplication, rd *radixv1.RadixDeployment, deploymentList []appsv1.Deployment, podList []corev1.Pod, hpaList []autoscalingv2.HorizontalPodAutoscaler) []*deploymentModels.Component {
	var components []*deploymentModels.Component
	// TODO: Implement

	for _, component := range rd.Spec.Components {
		components = append(components, buildComponent(&component, ra, rd, deploymentList, podList, hpaList))
	}

	for _, job := range rd.Spec.Jobs {
		components = append(components, buildComponent(&job, ra, rd, deploymentList, podList, hpaList))
	}

	return components
}

func buildComponent(radixComponent radixv1.RadixCommonDeployComponent, ra *radixv1.RadixApplication, rd *radixv1.RadixDeployment, deploymentList []appsv1.Deployment, podList []corev1.Pod, hpaList []autoscalingv2.HorizontalPodAutoscaler) *deploymentModels.Component {
	builder := deploymentModels.NewComponentBuilder().
		WithComponent(radixComponent).
		WithStatus(deploymentModels.ConsistentComponent).
		WithHorizontalScalingSummary(getHpaSummary(radixComponent, hpaList))

	componentPods := slice.FindAll(podList, predicate.IsPodForComponent(radixComponent.GetName()))

	if rd.Status.ActiveTo.IsZero() {
		builder.WithPodNames(slice.Map(componentPods, func(pod corev1.Pod) string { return pod.Name }))
		builder.WithRadixEnvironmentVariables(getRadixEnvironmentVariables(componentPods))
		builder.WithReplicaSummaryList(BuildReplicaSummaryList(componentPods))
		builder.WithStatus(getComponentStatus(radixComponent, ra, rd, componentPods))
		builder.WithAuxiliaryResource(getAuxiliaryResources(ra.Name, radixComponent, deploymentList, podList))
	}

	// TODO: Use radixComponent.GetType() instead?
	if jobComponent, ok := radixComponent.(*radixv1.RadixDeployJobComponent); ok {
		builder.WithSchedulerPort(jobComponent.SchedulerPort)
		if jobComponent.Payload != nil {
			builder.WithScheduledJobPayloadPath(jobComponent.Payload.Path)
		}
		builder.WithNotifications(jobComponent.Notifications)
	}

	// The only error that can be returned from DeploymentBuilder is related to errors from github.com/imdario/mergo
	// This type of error will only happen if incorrect objects (e.g. incompatible structs) are sent as arguments to mergo,
	// and we should consider to panic the error in the code calling merge.
	// For now we will panic the error here.
	component, err := builder.BuildComponent()
	if err != nil {
		panic(err)
	}
	return component
}

func getComponentStatus(component radixv1.RadixCommonDeployComponent, ra *radixv1.RadixApplication, rd *radixv1.RadixDeployment, pods []corev1.Pod) deploymentModels.ComponentStatus {
	environmentConfig := operatorapplicationconfig.GetComponentEnvironmentConfig(ra, rd.Spec.Environment, component.GetName())
	if component.GetType() == radixv1.RadixComponentTypeComponent {
		if runningReplicaDiffersFromConfig(environmentConfig, pods) &&
			!runningReplicaDiffersFromSpec(component, pods) &&
			len(pods) == 0 {
			return deploymentModels.StoppedComponent
		}
		if runningReplicaDiffersFromSpec(component, pods) {
			return deploymentModels.ComponentReconciling
		}
	} else if component.GetType() == radixv1.RadixComponentTypeJob {
		if len(pods) == 0 {
			return deploymentModels.StoppedComponent
		}
	}
	if runningReplicaIsOutdated(component, pods) {
		return deploymentModels.ComponentOutdated
	}
	restarted := component.GetEnvironmentVariables()[operatordefaults.RadixRestartEnvironmentVariable]
	if strings.EqualFold(restarted, "") {
		return deploymentModels.ConsistentComponent
	}
	restartedTime, err := commonutils.ParseTimestamp(restarted)
	if err != nil {
		// TODO: How should we handle invalid value for restarted time?
		logrus.Warnf("unable to parse restarted time %v: %v", restarted, err)
		return deploymentModels.ConsistentComponent
	}
	reconciledTime := rd.Status.Reconciled
	if reconciledTime.IsZero() || restartedTime.After(reconciledTime.Time) {
		return deploymentModels.ComponentRestarting
	}
	return deploymentModels.ConsistentComponent
}

func runningReplicaDiffersFromConfig(environmentConfig radixv1.RadixCommonEnvironmentConfig, actualPods []corev1.Pod) bool {
	actualPodsLength := len(actualPods)
	if commonutils.IsNil(environmentConfig) {
		return actualPodsLength != operatordeployment.DefaultReplicas
	}
	// No HPA config
	if environmentConfig.GetHorizontalScaling() == nil {
		if environmentConfig.GetReplicas() != nil {
			return actualPodsLength != *environmentConfig.GetReplicas()
		}
		return actualPodsLength != operatordeployment.DefaultReplicas
	}
	// With HPA config
	if environmentConfig.GetReplicas() != nil && *environmentConfig.GetReplicas() == 0 {
		return actualPodsLength != *environmentConfig.GetReplicas()
	}
	if environmentConfig.GetHorizontalScaling().MinReplicas != nil {
		return actualPodsLength < int(*environmentConfig.GetHorizontalScaling().MinReplicas) ||
			actualPodsLength > int(environmentConfig.GetHorizontalScaling().MaxReplicas)
	}
	return actualPodsLength < operatordeployment.DefaultReplicas ||
		actualPodsLength > int(environmentConfig.GetHorizontalScaling().MaxReplicas)
}

func runningReplicaDiffersFromSpec(component radixv1.RadixCommonDeployComponent, actualPods []corev1.Pod) bool {
	actualPodsLength := len(actualPods)
	// No HPA config
	if component.GetHorizontalScaling() == nil {
		if component.GetReplicas() != nil {
			return actualPodsLength != *component.GetReplicas()
		}
		return actualPodsLength != operatordeployment.DefaultReplicas
	}
	// With HPA config
	if component.GetReplicas() != nil && *component.GetReplicas() == 0 {
		return actualPodsLength != *component.GetReplicas()
	}
	if component.GetHorizontalScaling().MinReplicas != nil {
		return actualPodsLength < int(*component.GetHorizontalScaling().MinReplicas) ||
			actualPodsLength > int(component.GetHorizontalScaling().MaxReplicas)
	}
	return actualPodsLength < operatordeployment.DefaultReplicas ||
		actualPodsLength > int(component.GetHorizontalScaling().MaxReplicas)
}

func runningReplicaIsOutdated(component radixv1.RadixCommonDeployComponent, actualPods []corev1.Pod) bool {
	switch component.GetType() {
	case radixv1.RadixComponentTypeComponent:
		return runningComponentReplicaIsOutdated(component, actualPods)
	case radixv1.RadixComponentTypeJob:
		return false
	default:
		return false
	}
}

func runningComponentReplicaIsOutdated(component radixv1.RadixCommonDeployComponent, actualPods []corev1.Pod) bool {
	// Check if running component's image is not the same as active deployment image tag and that active rd image is equal to 'starting' component image tag
	componentIsInconsistent := false
	for _, pod := range actualPods {
		if pod.DeletionTimestamp != nil {
			// Pod is in termination phase
			continue
		}
		for _, container := range pod.Spec.Containers {
			if container.Image != component.GetImage() {
				// Container is running an outdated image
				componentIsInconsistent = true
			}
		}
	}

	return componentIsInconsistent
}

func getRadixEnvironmentVariables(pods []corev1.Pod) map[string]string {
	radixEnvironmentVariables := make(map[string]string)

	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			for _, envVariable := range container.Env {
				if operatorutils.IsRadixEnvVar(envVariable.Name) {
					radixEnvironmentVariables[envVariable.Name] = envVariable.Value
				}
			}
		}
	}

	return radixEnvironmentVariables
}
