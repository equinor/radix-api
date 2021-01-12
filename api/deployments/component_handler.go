package deployments

import (
	"fmt"
	"strings"

	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	"github.com/equinor/radix-api/api/utils"
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
	radixEnvVariablePrefix      = "RADIX_"
	defaultTargetCPUUtilization = int32(80)
)

// GetComponentsForDeployment Gets a list of components for a given deployment
func (deploy DeployHandler) GetComponentsForDeployment(appName string, deployment *deploymentModels.DeploymentSummary) ([]*deploymentModels.Component, error) {
	return deploy.getComponents(appName, deployment)
}

// GetComponentsForDeploymentName handler for GetDeployments
func (deploy DeployHandler) GetComponentsForDeploymentName(appName, deploymentID string) ([]*deploymentModels.Component, error) {
	deployments, err := deploy.GetDeploymentsForApplication(appName, false)
	if err != nil {
		return nil, err
	}

	for _, deployment := range deployments {
		if deployment.Name != deploymentID {
			continue
		}
		return deploy.getComponents(appName, deployment)
	}

	return nil, deploymentModels.NonExistingDeployment(nil, deploymentID)
}

func (deploy DeployHandler) getComponents(appName string, deployment *deploymentModels.DeploymentSummary) ([]*deploymentModels.Component, error) {
	envNs := crdUtils.GetEnvironmentNamespace(appName, deployment.Environment)
	rd, err := deploy.radixClient.RadixV1().RadixDeployments(envNs).Get(deployment.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	ra, _ := deploy.radixClient.RadixV1().RadixApplications(crdUtils.GetAppNamespace(appName)).Get(appName, metav1.GetOptions{})

	components := []*deploymentModels.Component{}
	for _, component := range rd.Spec.Components {

		environmentConfig := configUtils.GetComponentEnvironmentConfig(ra, deployment.Environment, component.Name)

		deploymentComponent, err :=
			GetComponentStateFromSpec(deploy.kubeClient, appName, deployment, rd.Status, environmentConfig, component)
		if err != nil {
			return nil, err
		}

		hpa, err := deploy.kubeClient.AutoscalingV1().HorizontalPodAutoscalers(envNs).Get(deploymentComponent.Name, metav1.GetOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return nil, err
		}
		if err == nil {
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
			deploymentComponent.HorizontalScalingSummary = &hpaSummary
		}

		components = append(components, deploymentComponent)
	}
	return components, nil
}

// GetComponentStateFromSpec Returns a component with the current state
func GetComponentStateFromSpec(
	client kubernetes.Interface,
	appName string,
	deployment *deploymentModels.DeploymentSummary,
	deploymentStatus v1.RadixDeployStatus,
	environmentConfig *v1.RadixEnvironmentConfig,
	component v1.RadixDeployComponent) (*deploymentModels.Component, error) {

	var environmentVariables map[string]string

	envNs := crdUtils.GetEnvironmentNamespace(appName, deployment.Environment)
	podNames := []string{}
	replicaSummaryList := []deploymentModels.ReplicaSummary{}
	status := deploymentModels.ConsistentComponent

	if deployment.ActiveTo == "" {
		// current active deployment - we get existing pods
		pods, err := getComponentPodsByNamespace(client, envNs, component.Name)
		if err != nil {
			return nil, err
		}
		podNames = getPodNames(pods)
		environmentVariables = getRadixEnvironmentVariables(pods)
		replicaSummaryList = getReplicaSummaryList(pods)

		status, err = getStatusOfActiveDeployment(component,
			deploymentStatus, environmentConfig, pods)
		if err != nil {
			return nil, err
		}
	}

	return deploymentModels.NewComponentBuilder().
		WithComponent(component).
		WithStatus(status).
		WithPodNames(podNames).
		WithReplicaSummaryList(replicaSummaryList).
		WithRadixEnvironmentVariables(environmentVariables).
		BuildComponent(), nil
}

func getComponentPodsByNamespace(client kubernetes.Interface, envNs, componentName string) ([]corev1.Pod, error) {
	pods, err := client.CoreV1().Pods(envNs).List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", kube.RadixComponentLabel, componentName),
	})
	if err != nil {
		log.Errorf("error getting pods: %v", err)
		return nil, err
	}

	return pods.Items, nil
}

func runningReplicaDiffersFromConfig(environmentConfig *v1.RadixEnvironmentConfig, actualPods []corev1.Pod) bool {
	actualPodsLength := len(actualPods)
	if environmentConfig != nil {
		// No HPA config
		if environmentConfig.HorizontalScaling == nil {
			if environmentConfig.Replicas != nil {
				return actualPodsLength != *environmentConfig.Replicas
			}
			return actualPodsLength != deployment.DefaultReplicas
		}
		// With HPA config
		if environmentConfig.Replicas != nil && *environmentConfig.Replicas == 0 {
			return actualPodsLength != *environmentConfig.Replicas
		}
		if environmentConfig.HorizontalScaling.MinReplicas != nil {
			return actualPodsLength < int(*environmentConfig.HorizontalScaling.MinReplicas) ||
				actualPodsLength > int(environmentConfig.HorizontalScaling.MaxReplicas)
		}
		return actualPodsLength < deployment.DefaultReplicas ||
			actualPodsLength > int(environmentConfig.HorizontalScaling.MaxReplicas)
	}
	return actualPodsLength != deployment.DefaultReplicas
}

func runningReplicaDiffersFromSpec(component v1.RadixDeployComponent, actualPods []corev1.Pod) bool {
	actualPodsLength := len(actualPods)
	// No HPA config
	if component.HorizontalScaling == nil {
		if component.Replicas != nil {
			return actualPodsLength != *component.Replicas
		}
		return actualPodsLength != deployment.DefaultReplicas
	}
	// With HPA config
	if component.Replicas != nil && *component.Replicas == 0 {
		return actualPodsLength != *component.Replicas
	}
	if component.HorizontalScaling.MinReplicas != nil {
		return actualPodsLength < int(*component.HorizontalScaling.MinReplicas) ||
			actualPodsLength > int(component.HorizontalScaling.MaxReplicas)
	}
	return actualPodsLength < deployment.DefaultReplicas ||
		actualPodsLength > int(component.HorizontalScaling.MaxReplicas)
}

func getPodNames(pods []corev1.Pod) []string {
	podNames := []string{}

	for _, pod := range pods {
		podNames = append(podNames, pod.GetName())
	}

	return podNames
}

func getRadixEnvironmentVariables(pods []corev1.Pod) map[string]string {
	radixEnvironmentVariables := make(map[string]string)

	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			for _, envVariable := range container.Env {
				if strings.HasPrefix(envVariable.Name, radixEnvVariablePrefix) {
					radixEnvironmentVariables[envVariable.Name] = envVariable.Value
				}
			}
		}
	}

	return radixEnvironmentVariables
}

func getReplicaSummaryList(pods []corev1.Pod) []deploymentModels.ReplicaSummary {
	replicaSummaryList := []deploymentModels.ReplicaSummary{}

	for _, pod := range pods {
		replicaSummary := deploymentModels.ReplicaSummary{}
		replicaSummary.Name = pod.GetName()
		if len(pod.Status.ContainerStatuses) > 0 {
			// We assume one component container per component pod
			containerStatus := pod.Status.ContainerStatuses[0]
			containerState := containerStatus.State

			// Set default Pending status
			replicaSummary.Status = deploymentModels.ReplicaStatus{Status: deploymentModels.Pending.String()}

			if containerState.Waiting != nil {
				replicaSummary.StatusMessage = containerState.Waiting.Message
				if !strings.EqualFold(containerState.Waiting.Reason, "ContainerCreating") {
					replicaSummary.Status = deploymentModels.ReplicaStatus{Status: deploymentModels.Failing.String()}
				}
			}
			if containerState.Running != nil {
				if containerStatus.Ready {
					replicaSummary.Status = deploymentModels.ReplicaStatus{Status: deploymentModels.Running.String()}
				} else {
					replicaSummary.Status = deploymentModels.ReplicaStatus{Status: deploymentModels.Starting.String()}
				}
			}
			if containerState.Terminated != nil {
				replicaSummary.Status = deploymentModels.ReplicaStatus{Status: deploymentModels.Terminated.String()}
				replicaSummary.StatusMessage = containerState.Terminated.Message
			}
		}
		replicaSummaryList = append(replicaSummaryList, replicaSummary)
	}

	return replicaSummaryList
}

func runningReplicaIsOutdated(component v1.RadixDeployComponent, actualPods []corev1.Pod) bool {
	// Check if running component's image is not the same as active deployment image tag and that active rd image is equal to 'starting' component image tag
	componentIsInconsistent := false
	for _, pod := range actualPods {
		if pod.DeletionTimestamp != nil {
			// Pod is in termination phase
			return false
		}
		for _, container := range pod.Spec.Containers {
			if container.Image != component.Image {
				// Container is running an outdate image
				componentIsInconsistent = true
			}
		}
	}

	return componentIsInconsistent
}

func getStatusOfActiveDeployment(
	component v1.RadixDeployComponent,
	deploymentStatus v1.RadixDeployStatus,
	environmentConfig *v1.RadixEnvironmentConfig,
	pods []corev1.Pod) (deploymentModels.ComponentStatus, error) {

	status := deploymentModels.ConsistentComponent

	if runningReplicaDiffersFromConfig(environmentConfig, pods) &&
		!runningReplicaDiffersFromSpec(component, pods) &&
		len(pods) == 0 {
		status = deploymentModels.StoppedComponent
	} else if runningReplicaIsOutdated(component, pods) {
		status = deploymentModels.ComponentOutdated
	} else if runningReplicaDiffersFromSpec(component, pods) {
		status = deploymentModels.ComponentReconciling
	} else {
		restarted := component.EnvironmentVariables[defaults.RadixRestartEnvironmentVariable]
		if !strings.EqualFold(restarted, "") {
			restartedTime, err := utils.ParseTimestamp(restarted)
			if err != nil {
				return status, err
			}

			reconciledTime := deploymentStatus.Reconciled
			if reconciledTime.IsZero() || restartedTime.After(reconciledTime.Time) {
				status = deploymentModels.ComponentRestarting
			}
		}
	}

	return status, nil
}
