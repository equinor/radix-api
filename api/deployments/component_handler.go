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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const radixEnvVariablePrefix = "RADIX_"

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
	return (environmentConfig != nil && environmentConfig.Replicas != nil && len(actualPods) != *environmentConfig.Replicas) ||
		((environmentConfig == nil || environmentConfig.Replicas == nil) && len(actualPods) != deployment.DefaultReplicas)
}

func runningReplicaDifferFromSpec(component v1.RadixDeployComponent, actualPods []corev1.Pod) bool {
	return (component.Replicas != nil && len(actualPods) != *component.Replicas) ||
		(component.Replicas == nil && len(actualPods) != deployment.DefaultReplicas)
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
			containerState := pod.Status.ContainerStatuses[0].State

			// Set default Pending status
			replicaSummary.Status = deploymentModels.ReplicaStatus{Status: deploymentModels.Pending.String()}

			if containerState.Waiting != nil {
				replicaSummary.StatusMessage = containerState.Waiting.Message
				if !strings.EqualFold(containerState.Waiting.Reason, "ContainerCreating") {
					replicaSummary.Status = deploymentModels.ReplicaStatus{Status: deploymentModels.Failing.String()}
				}
			}
			if containerState.Running != nil {
				replicaSummary.Status = deploymentModels.ReplicaStatus{Status: deploymentModels.Running.String()}
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

func getStatusOfActiveDeployment(
	component v1.RadixDeployComponent,
	deploymentStatus v1.RadixDeployStatus,
	environmentConfig *v1.RadixEnvironmentConfig,
	pods []corev1.Pod) (deploymentModels.ComponentStatus, error) {

	status := deploymentModels.ConsistentComponent

	if runningReplicaDifferFromConfig(environmentConfig, pods) &&
		!runningReplicaDifferFromSpec(component, pods) &&
		len(pods) == 0 {
		status = deploymentModels.StoppedComponent
	} else if runningReplicaDifferFromSpec(component, pods) {
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
