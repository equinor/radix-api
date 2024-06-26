package environments

import (
	"context"
	"strings"

	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	"github.com/equinor/radix-api/api/kubequery"
	"github.com/equinor/radix-api/api/models"
	"github.com/equinor/radix-api/api/utils/event"
	"github.com/equinor/radix-api/api/utils/labelselector"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
)

// getComponentStateFromSpec Returns a component with the current state
func getComponentStateFromSpec(
	ctx context.Context,
	kubeClient kubernetes.Interface,
	appName string,
	deployment *deploymentModels.DeploymentSummary,
	deploymentStatus v1.RadixDeployStatus,
	environmentConfig v1.RadixCommonEnvironmentConfig,
	component v1.RadixCommonDeployComponent,
	hpas []autoscalingv2.HorizontalPodAutoscaler,
	scaledObjects []v1alpha1.ScaledObject,
) (*deploymentModels.Component, error) {

	var componentPodNames []string
	var environmentVariables map[string]string
	var replicaSummaryList []deploymentModels.ReplicaSummary
	var auxResource deploymentModels.AuxiliaryResource
	var horizontalScalingSummary *deploymentModels.HorizontalScalingSummary

	envNs := crdUtils.GetEnvironmentNamespace(appName, deployment.Environment)
	status := deploymentModels.ConsistentComponent

	if deployment.ActiveTo == "" {
		// current active deployment - we get existing pods
		componentPods, err := getComponentPodsByNamespace(ctx, kubeClient, envNs, component.GetName())
		if err != nil {
			return nil, err
		}
		componentPodNames = getPodNames(componentPods)
		environmentVariables = getRadixEnvironmentVariables(componentPods)
		eventList, err := kubequery.GetEventsForEnvironment(ctx, kubeClient, appName, deployment.Environment)
		if err != nil {
			return nil, err
		}
		lastEventWarnings := event.ConvertToEventWarnings(eventList)
		replicaSummaryList = getReplicaSummaryList(componentPods, lastEventWarnings)
		auxResource, err = getAuxiliaryResources(ctx, kubeClient, appName, component, envNs)
		if err != nil {
			return nil, err
		}

		status, err = getStatusOfActiveDeployment(component,
			deploymentStatus, environmentConfig, componentPods)
		if err != nil {
			return nil, err
		}
	}

	componentBuilder := deploymentModels.NewComponentBuilder()
	if jobComponent, ok := component.(*v1.RadixDeployJobComponent); ok {
		componentBuilder.WithSchedulerPort(jobComponent.SchedulerPort)
		if jobComponent.Payload != nil {
			componentBuilder.WithScheduledJobPayloadPath(jobComponent.Payload.Path)
		}
		componentBuilder.WithNotifications(jobComponent.Notifications)
	}

	if component.GetType() == v1.RadixComponentTypeComponent {
		horizontalScalingSummary = models.GetHpaSummary(appName, component.GetName(), hpas, scaledObjects)
	}

	return componentBuilder.
		WithComponent(component).
		WithStatus(status).
		WithPodNames(componentPodNames).
		WithReplicaSummaryList(replicaSummaryList).
		WithRadixEnvironmentVariables(environmentVariables).
		WithAuxiliaryResource(auxResource).
		WithHorizontalScalingSummary(horizontalScalingSummary).
		BuildComponent()
}

func getPodNames(pods []corev1.Pod) []string {
	var names []string
	for _, pod := range pods {
		names = append(names, pod.GetName())
	}
	return names
}

func getComponentPodsByNamespace(ctx context.Context, client kubernetes.Interface, envNs, componentName string) ([]corev1.Pod, error) {
	var componentPods []corev1.Pod
	pods, err := client.CoreV1().Pods(envNs).List(ctx, metav1.ListOptions{
		LabelSelector: getLabelSelectorForComponentPods(componentName).String(),
	})
	if err != nil {
		return nil, err
	}

	for _, pod := range pods.Items {
		pod := pod

		// A previous version of the job-scheduler added the "radix-job-type" label to job pods.
		// For backward compatibility, we need to ignore these pods in the list of pods returned for a component
		if _, isScheduledJobPod := pod.GetLabels()[kube.RadixJobTypeLabel]; isScheduledJobPod {
			continue
		}

		// Ignore pods related to jobs created from RadixBatch
		if _, isRadixBatchJobPod := pod.GetLabels()[kube.RadixBatchNameLabel]; isRadixBatchJobPod {
			continue
		}

		componentPods = append(componentPods, pod)
	}

	return componentPods, nil
}

func getLabelSelectorForComponentPods(componentName string) labels.Selector {
	componentNameRequirement, _ := labels.NewRequirement(kube.RadixComponentLabel, selection.Equals, []string{componentName})
	notJobAuxRequirement, _ := labels.NewRequirement(kube.RadixPodIsJobAuxObjectLabel, selection.DoesNotExist, []string{})
	return labels.NewSelector().Add(*componentNameRequirement, *notJobAuxRequirement)
}

func runningReplicaDiffersFromConfig(environmentConfig v1.RadixCommonEnvironmentConfig, actualPods []corev1.Pod) bool {
	actualPodsLength := len(actualPods)
	if radixutils.IsNil(environmentConfig) {
		return actualPodsLength != deployment.DefaultReplicas
	}
	// No HPA config
	if environmentConfig.GetHorizontalScaling() == nil {
		if environmentConfig.GetReplicas() != nil {
			return actualPodsLength != *environmentConfig.GetReplicas()
		}
		return actualPodsLength != deployment.DefaultReplicas
	}
	// With HPA config
	if environmentConfig.GetReplicas() != nil && *environmentConfig.GetReplicas() == 0 {
		return actualPodsLength != *environmentConfig.GetReplicas()
	}
	if environmentConfig.GetHorizontalScaling().MinReplicas != nil {
		return actualPodsLength < int(*environmentConfig.GetHorizontalScaling().MinReplicas) ||
			actualPodsLength > int(environmentConfig.GetHorizontalScaling().MaxReplicas)
	}
	return actualPodsLength < deployment.DefaultReplicas ||
		actualPodsLength > int(environmentConfig.GetHorizontalScaling().MaxReplicas)
}

func runningReplicaDiffersFromSpec(component v1.RadixCommonDeployComponent, actualPods []corev1.Pod) bool {
	actualPodsLength := len(actualPods)
	// No HPA config
	if component.GetHorizontalScaling() == nil {
		if component.GetReplicas() != nil {
			return actualPodsLength != *component.GetReplicas()
		}
		return actualPodsLength != deployment.DefaultReplicas
	}
	// With HPA config
	if component.GetReplicas() != nil && *component.GetReplicas() == 0 {
		return actualPodsLength != *component.GetReplicas()
	}
	if component.GetHorizontalScaling().MinReplicas != nil {
		return actualPodsLength < int(*component.GetHorizontalScaling().MinReplicas) ||
			actualPodsLength > int(component.GetHorizontalScaling().MaxReplicas)
	}
	return actualPodsLength < deployment.DefaultReplicas ||
		actualPodsLength > int(component.GetHorizontalScaling().MaxReplicas)
}

func getRadixEnvironmentVariables(pods []corev1.Pod) map[string]string {
	radixEnvironmentVariables := make(map[string]string)

	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			for _, envVariable := range container.Env {
				if crdUtils.IsRadixEnvVar(envVariable.Name) {
					radixEnvironmentVariables[envVariable.Name] = envVariable.Value
				}
			}
		}
	}

	return radixEnvironmentVariables
}

func getReplicaSummaryList(pods []corev1.Pod, lastEventWarnings event.LastEventWarnings) []deploymentModels.ReplicaSummary {
	return slice.Map(pods, func(pod corev1.Pod) deploymentModels.ReplicaSummary {
		return deploymentModels.GetReplicaSummary(pod, lastEventWarnings[pod.GetName()])
	})
}

func getAuxiliaryResources(ctx context.Context, kubeClient kubernetes.Interface, appName string, component v1.RadixCommonDeployComponent, envNamespace string) (auxResource deploymentModels.AuxiliaryResource, err error) {
	if auth := component.GetAuthentication(); component.IsPublic() && auth != nil && auth.OAuth2 != nil {
		auxResource.OAuth2, err = getOAuth2AuxiliaryResource(ctx, kubeClient, appName, component.GetName(), envNamespace)
		if err != nil {
			return
		}
	}

	return
}

func getOAuth2AuxiliaryResource(ctx context.Context, kubeClient kubernetes.Interface, appName, componentName, envNamespace string) (*deploymentModels.OAuth2AuxiliaryResource, error) {
	var oauth2Resource deploymentModels.OAuth2AuxiliaryResource
	oauthDeployment, err := getAuxiliaryResourceDeployment(ctx, kubeClient, appName, componentName, envNamespace, defaults.OAuthProxyAuxiliaryComponentType)
	if err != nil {
		return nil, err
	}
	if oauthDeployment != nil {
		oauth2Resource.Deployment = *oauthDeployment
	}

	return &oauth2Resource, nil
}

func getAuxiliaryResourceDeployment(ctx context.Context, kubeClient kubernetes.Interface, appName, componentName, envNamespace, auxType string) (*deploymentModels.AuxiliaryResourceDeployment, error) {
	var auxResourceDeployment deploymentModels.AuxiliaryResourceDeployment

	selector := labelselector.ForAuxiliaryResource(appName, componentName, auxType).String()
	deployments, err := kubeClient.AppsV1().Deployments(envNamespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, err
	}
	if len(deployments.Items) == 0 {
		auxResourceDeployment.Status = deploymentModels.ComponentReconciling.String()
		return &auxResourceDeployment, nil
	}
	deployment := deployments.Items[0]

	pods, err := kubeClient.CoreV1().Pods(envNamespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, err
	}
	auxResourceDeployment.ReplicaList = getReplicaSummaryList(pods.Items, nil)
	auxResourceDeployment.Status = deploymentModels.ComponentStatusFromDeployment(&deployment).String()
	return &auxResourceDeployment, nil
}

func runningReplicaIsOutdated(component v1.RadixCommonDeployComponent, actualPods []corev1.Pod) bool {
	switch component.GetType() {
	case v1.RadixComponentTypeComponent:
		return runningComponentReplicaIsOutdated(component, actualPods)
	case v1.RadixComponentTypeJob:
		return false
	default:
		return false
	}
}

func runningComponentReplicaIsOutdated(component v1.RadixCommonDeployComponent, actualPods []corev1.Pod) bool {
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

func getStatusOfActiveDeployment(
	component v1.RadixCommonDeployComponent,
	deploymentStatus v1.RadixDeployStatus,
	environmentConfig v1.RadixCommonEnvironmentConfig,
	pods []corev1.Pod) (deploymentModels.ComponentStatus, error) {

	if component.GetType() == v1.RadixComponentTypeComponent {
		if runningReplicaDiffersFromConfig(environmentConfig, pods) &&
			!runningReplicaDiffersFromSpec(component, pods) &&
			len(pods) == 0 {
			return deploymentModels.StoppedComponent, nil
		}
		if runningReplicaDiffersFromSpec(component, pods) {
			return deploymentModels.ComponentReconciling, nil
		}
	} else if component.GetType() == v1.RadixComponentTypeJob {
		if len(pods) == 0 {
			return deploymentModels.StoppedComponent, nil
		}
	}
	if runningReplicaIsOutdated(component, pods) {
		return deploymentModels.ComponentOutdated, nil
	}
	restarted := component.GetEnvironmentVariables()[defaults.RadixRestartEnvironmentVariable]
	if strings.EqualFold(restarted, "") {
		return deploymentModels.ConsistentComponent, nil
	}
	restartedTime, err := radixutils.ParseTimestamp(restarted)
	if err != nil {
		return deploymentModels.ConsistentComponent, err
	}
	reconciledTime := deploymentStatus.Reconciled
	if reconciledTime.IsZero() || restartedTime.After(reconciledTime.Time) {
		return deploymentModels.ComponentRestarting, nil
	}
	return deploymentModels.ConsistentComponent, nil
}
