package deployments

import (
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	components := []*deploymentModels.Component{}
	for _, component := range rd.Spec.Components {
		var environmentVariables map[string]string
		podNames := []string{}
		replicaSummaryList := []deploymentModels.ReplicaSummary{}

		if deployment.ActiveTo == "" {
			// current active deployment - we get existing pods
			pods, err := deploy.getComponentPodsByNamespace(envNs, component.Name)
			if err != nil {
				return nil, err
			}
			podNames = getPodNames(pods)
			environmentVariables = getRadixEnvironmentVariables(pods)
			replicaSummaryList = getReplicaSummaryList(pods)
		}

		deploymentComponent := deploymentModels.NewComponentBuilder().
			WithComponent(component).
			WithPodNames(podNames).
			WithReplicaSummaryList(replicaSummaryList).
			WithRadixEnvironmentVariables(environmentVariables).
			BuildComponent()

		components = append(components, deploymentComponent)
	}
	return components, nil
}

func (deploy DeployHandler) getComponentPodsByNamespace(envNs, componentName string) ([]corev1.Pod, error) {
	pods, err := deploy.kubeClient.CoreV1().Pods(envNs).List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", kube.RadixComponentLabel, componentName),
	})
	if err != nil {
		log.Errorf("error getting pods: %v", err)
		return nil, err
	}

	return pods.Items, nil
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
			if containerState.Waiting != nil {
				replicaSummary.StatusMessage = containerState.Waiting.Message
				if strings.EqualFold(containerState.Waiting.Reason, "ContainerCreating") {
					replicaSummary.Status = deploymentModels.ReplicaStatus{Status: deploymentModels.Pending.String()}
				} else {
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
