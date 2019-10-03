package environments

import (
	"fmt"
	"strings"
	"time"

	"github.com/equinor/radix-api/api/deployments"
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	environmentModels "github.com/equinor/radix-api/api/environments/models"
	"github.com/equinor/radix-api/api/utils"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// StopComponent Stops a component
func (eh EnvironmentHandler) StopComponent(appName, envName, componentName string) error {
	envNs := crdUtils.GetEnvironmentNamespace(appName, envName)
	deployment, err := eh.deployHandler.GetLatestDeploymentForApplicationEnvironment(appName, envName)
	if err != nil {
		return err
	}

	rd, err := eh.radixclient.RadixV1().RadixDeployments(envNs).Get(deployment.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	index, componentToPatch := getDeploymentComponent(rd, componentName)
	if componentToPatch == nil {
		return environmentModels.NonExistingComponent(appName, componentName)
	}

	componentState, err := deployments.GetComponentStateFromSpec(eh.client, appName, deployment, rd.Status, *componentToPatch)
	if err != nil {
		return err
	}

	if !strings.EqualFold(componentState.Status, deploymentModels.ConsistentComponent.String()) {
		return environmentModels.CannotStopComponent(appName, componentName, componentState.Status)
	}

	log.Infof("Stopping component %s, %s", componentName, appName)
	noReplicas := 0
	err = eh.patchReplicasOnRD(appName, rd.GetNamespace(), rd.GetName(), index, &noReplicas)
	if err != nil {
		return err
	}

	return nil
}

// StartComponent Starts a component
func (eh EnvironmentHandler) StartComponent(appName, envName, componentName string) error {
	envNs := crdUtils.GetEnvironmentNamespace(appName, envName)
	deployment, err := eh.deployHandler.GetLatestDeploymentForApplicationEnvironment(appName, envName)
	if err != nil {
		return err
	}

	rd, err := eh.radixclient.RadixV1().RadixDeployments(envNs).Get(deployment.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	ra, _ := eh.radixclient.RadixV1().RadixApplications(crdUtils.GetAppNamespace(appName)).Get(appName, metav1.GetOptions{})
	replicas, err := getReplicasForComponentInEnvironment(ra, envName, componentName)
	if err != nil {
		return err
	}

	index, componentToPatch := getDeploymentComponent(rd, componentName)
	if componentToPatch == nil {
		return environmentModels.NonExistingComponent(appName, componentName)
	}

	componentState, err := deployments.GetComponentStateFromSpec(eh.client, appName, deployment, rd.Status, *componentToPatch)
	if err != nil {
		return err
	}

	if !strings.EqualFold(componentState.Status, deploymentModels.ConsistentComponent.String()) {
		return environmentModels.CannotStartComponent(appName, componentName, componentState.Status)
	}

	log.Infof("Starting component %s, %s", componentName, appName)
	err = eh.patchReplicasOnRD(appName, rd.GetNamespace(), rd.GetName(), index, replicas)
	if err != nil {
		return err
	}

	return nil
}

// RestartComponent Restarts a component
func (eh EnvironmentHandler) RestartComponent(appName, envName, componentName string) error {
	log.Infof("Restarting component %s, %s", componentName, appName)
	envNs := crdUtils.GetEnvironmentNamespace(appName, envName)
	deployment, err := eh.deployHandler.GetLatestDeploymentForApplicationEnvironment(appName, envName)
	if err != nil {
		return err
	}

	rd, err := eh.radixclient.RadixV1().RadixDeployments(envNs).Get(deployment.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	index, componentToUpdate := getDeploymentComponent(rd, componentName)
	if componentToUpdate == nil {
		return environmentModels.NonExistingComponent(appName, componentName)
	}

	componentState, err := deployments.GetComponentStateFromSpec(eh.client, appName, deployment, rd.Status, *componentToUpdate)
	if err != nil {
		return err
	}

	if !strings.EqualFold(componentState.Status, deploymentModels.ConsistentComponent.String()) {
		return environmentModels.CannotStartComponent(appName, componentName, componentState.Status)
	}

	err = eh.updateRestartEnvironmentVariableOnRD(appName, rd, index, componentToUpdate)
	if err != nil {
		return err
	}

	return nil
}

func getReplicasForComponentInEnvironment(ra *v1.RadixApplication, envName, componentName string) (*int, error) {
	componentDefinition := getComponentDefinition(ra, componentName)

	environmentConfig := getEnvironment(componentDefinition, envName)
	if environmentConfig != nil {
		return environmentConfig.Replicas, nil
	}

	return nil, environmentModels.NonExistingComponent(ra.GetName(), componentName)
}

func getComponentDefinition(ra *v1.RadixApplication, name string) *v1.RadixComponent {
	for _, component := range ra.Spec.Components {
		if strings.EqualFold(component.Name, name) {
			return &component
		}
	}

	return nil
}

func getDeploymentComponent(rd *v1.RadixDeployment, name string) (int, *v1.RadixDeployComponent) {
	for index, component := range rd.Spec.Components {
		if strings.EqualFold(component.Name, name) {
			return index, &component
		}
	}

	return -1, nil
}

func getEnvironment(component *v1.RadixComponent, envName string) *v1.RadixEnvironmentConfig {
	if component != nil {
		for _, environment := range component.EnvironmentConfig {
			if strings.EqualFold(environment.Environment, envName) {
				return &environment
			}
		}
	}

	return nil
}

func (eh EnvironmentHandler) updateRestartEnvironmentVariableOnRD(appName string,
	rd *v1.RadixDeployment, componentIndex int, componentToUpdate *v1.RadixDeployComponent) error {
	environmentVariables := componentToUpdate.EnvironmentVariables
	if environmentVariables == nil {
		environmentVariables = make(map[string]string)
	}

	environmentVariables[defaults.RadixRestartEnvironmentVariable] = utils.FormatTimestamp(time.Now())
	rd.Spec.Components[componentIndex].EnvironmentVariables = environmentVariables
	_, err := eh.radixclient.RadixV1().RadixDeployments(rd.GetNamespace()).Update(rd)
	if err != nil {
		return fmt.Errorf("Failed to update deployment object: %v", err)
	}

	return nil
}

func (eh EnvironmentHandler) patchReplicasOnRD(appName, namespace, name string, componentIndex int, replicas *int) error {
	var patchBytes []byte

	newReplica := 1
	if replicas != nil {
		newReplica = *replicas
	}

	patchJSON := fmt.Sprintf(`[{"op": "replace", "path": "/spec/components/%d/replicas","value": %d}]`, componentIndex, newReplica)
	patchBytes = []byte(patchJSON)

	if patchBytes != nil {
		_, err := eh.radixclient.RadixV1().RadixDeployments(namespace).Patch(name, types.JSONPatchType, patchBytes)
		if err != nil {
			return fmt.Errorf("Failed to patch deployment object: %v", err)
		}
	}

	return nil
}
