package environments

import (
	"encoding/json"
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
	jsonpatch "github.com/evanphx/json-patch"
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
	err = eh.patchReplicasOnRD(appName, rd, index, &noReplicas)
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

	if !strings.EqualFold(componentState.Status, deploymentModels.StoppedComponent.String()) {
		return environmentModels.CannotStartComponent(appName, componentName, componentState.Status)
	}

	log.Infof("Starting component %s, %s", componentName, appName)
	err = eh.patchReplicasOnRD(appName, rd, index, replicas)
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
		return environmentModels.CannotRestartComponent(appName, componentName, componentState.Status)
	}

	err = eh.patchRestartEnvironmentVariableOnRD(appName, rd, index, componentToUpdate)
	if err != nil {
		return err
	}

	return nil
}

func getReplicasForComponentInEnvironment(ra *v1.RadixApplication, envName, componentName string) (*int, error) {
	componentDefinition := getComponentDefinition(ra, componentName)

	if componentDefinition == nil {
		return nil, environmentModels.NonExistingComponent(ra.GetName(), componentName)
	}

	environmentConfig := getEnvironment(componentDefinition, envName)
	if environmentConfig != nil {
		return environmentConfig.Replicas, nil
	}

	return nil, nil
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

func (eh EnvironmentHandler) patchRestartEnvironmentVariableOnRD(appName string,
	rd *v1.RadixDeployment, componentIndex int, componentToUpdate *v1.RadixDeployComponent) error {

	oldJSON, err := json.Marshal(rd)
	if err != nil {
		return err
	}

	environmentVariables := componentToUpdate.EnvironmentVariables
	if environmentVariables == nil {
		environmentVariables = make(map[string]string)
	}

	environmentVariables[defaults.RadixRestartEnvironmentVariable] = utils.FormatTimestamp(time.Now())
	rd.Spec.Components[componentIndex].EnvironmentVariables = environmentVariables

	newJSON, err := json.Marshal(rd)
	if err != nil {
		return err
	}

	err = eh.patch(rd.GetNamespace(), rd.GetName(), oldJSON, newJSON)
	if err != nil {
		return err
	}

	return nil
}

func (eh EnvironmentHandler) patchReplicasOnRD(appName string, rd *v1.RadixDeployment, componentIndex int, replicas *int) error {
	oldJSON, err := json.Marshal(rd)
	if err != nil {
		return err
	}

	newReplica := 1
	if replicas != nil {
		newReplica = *replicas
	}

	rd.Spec.Components[componentIndex].Replicas = &newReplica
	newJSON, err := json.Marshal(rd)
	if err != nil {
		return err
	}

	err = eh.patch(rd.GetNamespace(), rd.GetName(), oldJSON, newJSON)
	if err != nil {
		return err
	}

	return nil
}

func (eh EnvironmentHandler) patch(namespace, name string, oldJSON, newJSON []byte) error {
	patchBytes, err := jsonpatch.CreateMergePatch(oldJSON, newJSON)

	if err != nil {
		log.Fatalln(err)
	}

	if patchBytes != nil {
		_, err := eh.radixclient.RadixV1().RadixDeployments(namespace).Patch(name, types.MergePatchType, patchBytes)
		if err != nil {
			return fmt.Errorf("Failed to patch deployment object: %v", err)
		}
	}

	return nil
}
