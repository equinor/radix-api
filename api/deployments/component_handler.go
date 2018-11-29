package deployments

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	deploymentModels "github.com/statoil/radix-api/api/deployments/models"
	crdUtils "github.com/statoil/radix-operator/pkg/apis/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HandleGetComponents handler for GetDeployments
func (deploy DeployHandler) HandleGetComponents(appName, deploymentID string) ([]*deploymentModels.Component, error) {
	deployments, err := deploy.HandleGetDeployments(appName, "", false)
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
		podNames := []string{}
		if deployment.ActiveTo == "" {
			// current active deployment - we get existing pods
			podNames, err = deploy.getPodNames(envNs, component.Name)
			if err != nil {
				return nil, err
			}
		}

		deploymentComponent := deploymentModels.NewComponentBuilder().
			WithComponent(component).
			WithPodNames(podNames).
			BuildComponent()

		components = append(components, deploymentComponent)
	}
	return components, nil
}

func (deploy DeployHandler) getPodNames(envNs, componentName string) ([]string, error) {
	podNames := []string{}
	pods, err := deploy.kubeClient.CoreV1().Pods(envNs).List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("radixComponent=%s", componentName),
	})
	if err != nil {
		log.Errorf("error getting pod names: %v", err)
		return podNames, err
	}

	for _, pod := range pods.Items {
		podNames = append(podNames, pod.GetName())
	}
	return podNames, nil
}
