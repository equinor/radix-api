package deployments

import (
	"context"
	"fmt"
	"strings"

	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	"github.com/equinor/radix-api/api/utils/labelselector"
	radixutils "github.com/equinor/radix-common/utils"
	configUtils "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	defaultTargetCPUUtilization = int32(80)
)

// GetComponentsForDeployment Gets a list of components for a given deployment
func (deploy *deployHandler) GetComponentsForDeployment(appName string, deployment *deploymentModels.DeploymentSummary) ([]*deploymentModels.Component, error) {
	return deploy.getComponents(appName, deployment)
}

// GetComponentsForDeploymentName handler for GetDeployments
func (deploy *deployHandler) GetComponentsForDeploymentName(appName, deploymentID string) ([]*deploymentModels.Component, error) {
	deployments, err := deploy.GetDeploymentsForApplication(appName, false)
	if err != nil {
		return nil, err
	}

	for _, depl := range deployments {
		if depl.Name != deploymentID {
			continue
		}
		return deploy.getComponents(appName, depl)
	}

	return nil, deploymentModels.NonExistingDeployment(nil, deploymentID)
}

func (deploy *deployHandler) getComponents(appName string, deployment *deploymentModels.DeploymentSummary) ([]*deploymentModels.Component, error) {
	envNs := crdUtils.GetEnvironmentNamespace(appName, deployment.Environment)
	rd, err := deploy.radixClient.RadixV1().RadixDeployments(envNs).Get(context.TODO(), deployment.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	ra, _ := deploy.radixClient.RadixV1().RadixApplications(crdUtils.GetAppNamespace(appName)).Get(context.TODO(), appName, metav1.GetOptions{})
	var components []*deploymentModels.Component

	for _, component := range rd.Spec.Components {
		componentModel, err := deploy.getComponent(&component, ra, rd, deployment)
		if err != nil {
			return nil, err
		}
		components = append(components, componentModel)
	}

	for _, component := range rd.Spec.Jobs {
		componentModel, err := deploy.getComponent(&component, ra, rd, deployment)
		if err != nil {
			return nil, err
		}
		components = append(components, componentModel)
	}

	return components, nil
}

func (deploy *deployHandler) getComponent(component v1.RadixCommonDeployComponent, ra *v1.RadixApplication, rd *v1.RadixDeployment, deployment *deploymentModels.DeploymentSummary) (*deploymentModels.Component, error) {
	envNs := crdUtils.GetEnvironmentNamespace(ra.Name, deployment.Environment)

	// TODO: Add interface for RA + EnvConfig
	environmentConfig := configUtils.GetComponentEnvironmentConfig(ra, deployment.Environment, component.GetName())

	deploymentComponent, err :=
		GetComponentStateFromSpec(deploy.kubeClient, ra.Name, deployment, rd.Status, environmentConfig, component)
	if err != nil {
		return nil, err
	}
	if component.GetType() == v1.RadixComponentTypeComponent {
		hpaSummary, err := deploy.getHpaSummary(component, envNs)
		if err != nil {
			return nil, err
		}
		deploymentComponent.HorizontalScalingSummary = hpaSummary
	}
	return deploymentComponent, nil
}

func (deploy *deployHandler) getHpaSummary(component v1.RadixCommonDeployComponent, envNs string) (*deploymentModels.HorizontalScalingSummary, error) {
	hpa, err := deploy.kubeClient.AutoscalingV1().HorizontalPodAutoscalers(envNs).Get(context.TODO(), component.GetName(), metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	minReplicas := int32(1)
	if hpa.Spec.MinReplicas != nil {
		minReplicas = *hpa.Spec.MinReplicas
	}
	maxReplicas := hpa.Spec.MaxReplicas
	currentCPUUtil := int32(0)
	if hpa.Status.CurrentCPUUtilizationPercentage != nil {
		currentCPUUtil = *hpa.Status.CurrentCPUUtilizationPercentage
	}
	targetCPUUtil := defaultTargetCPUUtilization
	if hpa.Spec.TargetCPUUtilizationPercentage != nil {
		targetCPUUtil = *hpa.Spec.TargetCPUUtilizationPercentage
	}
	hpaSummary := deploymentModels.HorizontalScalingSummary{
		MinReplicas:                     minReplicas,
		MaxReplicas:                     maxReplicas,
		CurrentCPUUtilizationPercentage: currentCPUUtil,
		TargetCPUUtilizationPercentage:  targetCPUUtil,
	}
	return &hpaSummary, nil
}

// GetComponentStateFromSpec Returns a component with the current state
func GetComponentStateFromSpec(
	kubeClient kubernetes.Interface,
	appName string,
	deployment *deploymentModels.DeploymentSummary,
	deploymentStatus v1.RadixDeployStatus,
	environmentConfig v1.RadixCommonEnvironmentConfig,
	component v1.RadixCommonDeployComponent) (*deploymentModels.Component, error) {

	var environmentVariables map[string]string

	envNs := crdUtils.GetEnvironmentNamespace(appName, deployment.Environment)
	var componentPodNames []string

	var replicaSummaryList []deploymentModels.ReplicaSummary
	var auxResource deploymentModels.AuxiliaryResource
	status := deploymentModels.ConsistentComponent

	if deployment.ActiveTo == "" {
		// current active deployment - we get existing pods
		componentPods, err := getComponentPodsByNamespace(kubeClient, envNs, component.GetName())
		if err != nil {
			return nil, err
		}
		componentPodNames = getPodNames(componentPods)
		environmentVariables = getRadixEnvironmentVariables(componentPods)
		replicaSummaryList = getReplicaSummaryList(componentPods)
		auxResource, err = getAuxiliaryResources(kubeClient, appName, component, envNs)
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
	}

	return componentBuilder.
		WithComponent(component).
		WithStatus(status).
		WithPodNames(componentPodNames).
		WithReplicaSummaryList(replicaSummaryList).
		WithRadixEnvironmentVariables(environmentVariables).
		WithAuxiliaryResource(auxResource).
		BuildComponent()

}

func getPodNames(pods []corev1.Pod) []string {
	var names []string
	for _, pod := range pods {
		names = append(names, pod.GetName())
	}
	return names
}

func getComponentPodsByNamespace(client kubernetes.Interface, envNs, componentName string) ([]corev1.Pod, error) {
	var componentPods []corev1.Pod
	pods, err := client.CoreV1().Pods(envNs).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", kube.RadixComponentLabel, componentName),
	})
	if err != nil {
		log.Errorf("error getting pods: %v", err)
		return nil, err
	}

	for _, pod := range pods.Items {
		pod := pod

		// A previous version of the job-scheduler added the "radix-component" label to job pods.
		// For backward compatibility, we need to ignore these pods in the list of pods returned for a component
		if _, isScheduledJobPod := pod.GetLabels()[kube.RadixJobTypeLabel]; !isScheduledJobPod {
			componentPods = append(componentPods, pod)
		}
	}

	return componentPods, nil
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

func getReplicaSummaryList(pods []corev1.Pod) []deploymentModels.ReplicaSummary {
	replicaSummaryList := make([]deploymentModels.ReplicaSummary, 0, len(pods))

	for _, pod := range pods {
		replicaSummaryList = append(replicaSummaryList, deploymentModels.GetReplicaSummary(pod))
	}

	return replicaSummaryList
}

func getAuxiliaryResources(kubeClient kubernetes.Interface, appName string, component v1.RadixCommonDeployComponent, envNamespace string) (auxResource deploymentModels.AuxiliaryResource, err error) {
	if auth := component.GetAuthentication(); component.IsPublic() && auth != nil && auth.OAuth2 != nil {
		auxResource.OAuth2, err = getOAuth2AuxiliaryResource(kubeClient, appName, component.GetName(), envNamespace)
		if err != nil {
			return
		}
	}

	return
}

func getOAuth2AuxiliaryResource(kubeClient kubernetes.Interface, appName, componentName, envNamespace string) (*deploymentModels.OAuth2AuxiliaryResource, error) {
	var oauth2Resource deploymentModels.OAuth2AuxiliaryResource
	oauthDeployment, err := getAuxiliaryResourceDeployment(kubeClient, appName, componentName, envNamespace, defaults.OAuthProxyAuxiliaryComponentType)
	if err != nil {
		return nil, err
	}
	if oauthDeployment != nil {
		oauth2Resource.Deployment = *oauthDeployment
	}

	return &oauth2Resource, nil

}

func getAuxiliaryResourceDeployment(kubeClient kubernetes.Interface, appName, componentName, envNamespace, auxType string) (*deploymentModels.AuxiliaryResourceDeployment, error) {
	var auxResourceDeployment deploymentModels.AuxiliaryResourceDeployment

	selector := labelselector.ForAuxiliaryResource(appName, componentName, auxType).String()
	deployments, err := kubeClient.AppsV1().Deployments(envNamespace).List(context.TODO(), metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, err
	}
	if len(deployments.Items) == 0 {
		auxResourceDeployment.Status = deploymentModels.ComponentReconciling.String()
		return &auxResourceDeployment, nil
	}
	deployment := deployments.Items[0]

	pods, err := kubeClient.CoreV1().Pods(envNamespace).List(context.TODO(), metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, err
	}
	auxResourceDeployment.ReplicaList = getReplicaSummaryList(pods.Items)
	auxResourceDeployment.Status = deploymentModels.ComponentStatusFromDeployment(&deployment).String()
	return &auxResourceDeployment, nil
}

func runningReplicaIsOutdated(component v1.RadixCommonDeployComponent, actualPods []corev1.Pod) bool {
	switch component.GetType() {
	case v1.RadixComponentTypeComponent:
		return runningComponentReplicaIsOutdated(component, actualPods)
	case v1.RadixComponentTypeJobScheduler:
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
	} else if component.GetType() == v1.RadixComponentTypeJobScheduler {
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
