package deployments

import (
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	deploymentModels "github.com/statoil/radix-api/api/deployments/models"
	crdUtils "github.com/statoil/radix-operator/pkg/apis/utils"
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

		if deployment.ActiveTo == "" {
			// current active deployment - we get existing pods
			podNames, environmentVariables, err = deploy.getPodNamesAndEnvironmentVariables(envNs, component.Name)
			if err != nil {
				return nil, err
			}
		}

		deploymentComponent := deploymentModels.NewComponentBuilder().
			WithComponent(component).
			WithPodNames(podNames).
			WithRadixEnvironmentVariables(environmentVariables).
			BuildComponent()

		components = append(components, deploymentComponent)
	}
	return components, nil
}

func (deploy DeployHandler) getPodNamesAndEnvironmentVariables(envNs, componentName string) ([]string, map[string]string, error) {
	podNames := []string{}
	radixEnvironmentVariables := make(map[string]string)

	pods, err := deploy.kubeClient.CoreV1().Pods(envNs).List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("radix-component=%s", componentName),
	})
	if err != nil {
		log.Errorf("error getting pod names: %v", err)
		return podNames, radixEnvironmentVariables, err
	}

	for _, pod := range pods.Items {
		podNames = append(podNames, pod.GetName())

		for _, container := range pod.Spec.Containers {
			for _, envVariable := range container.Env {
				if strings.HasPrefix(envVariable.Name, radixEnvVariablePrefix) {
					radixEnvironmentVariables[envVariable.Name] = envVariable.Value
				}
			}
		}
	}
	return podNames, radixEnvironmentVariables, nil
}
