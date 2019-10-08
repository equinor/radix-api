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
	configUtils "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	deployUtils "github.com/equinor/radix-operator/pkg/apis/deployment"
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

	index, componentToPatch := deployUtils.GetDeploymentComponent(rd, componentName)
	if componentToPatch == nil {
		return environmentModels.NonExistingComponent(appName, componentName)
	}

	ra, _ := eh.radixclient.RadixV1().RadixApplications(crdUtils.GetAppNamespace(appName)).Get(appName, metav1.GetOptions{})
	environmentConfig := configUtils.GetComponentEnvironmentConfig(ra, envName, componentName)

	componentState, err := deployments.GetComponentStateFromSpec(eh.client, appName, deployment, rd.Status, environmentConfig, *componentToPatch)
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

	index, componentToPatch := deployUtils.GetDeploymentComponent(rd, componentName)
	if componentToPatch == nil {
		return environmentModels.NonExistingComponent(appName, componentName)
	}

	ra, _ := eh.radixclient.RadixV1().RadixApplications(crdUtils.GetAppNamespace(appName)).Get(appName, metav1.GetOptions{})
	environmentConfig := configUtils.GetComponentEnvironmentConfig(ra, envName, componentName)

	replicas, err := getReplicasForComponentInEnvironment(environmentConfig)
	if err != nil {
		return err
	}

	componentState, err := deployments.GetComponentStateFromSpec(eh.client, appName, deployment, rd.Status, environmentConfig, *componentToPatch)
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

	index, componentToPatch := deployUtils.GetDeploymentComponent(rd, componentName)
	if componentToPatch == nil {
		return environmentModels.NonExistingComponent(appName, componentName)
	}

	ra, _ := eh.radixclient.RadixV1().RadixApplications(crdUtils.GetAppNamespace(appName)).Get(appName, metav1.GetOptions{})
	environmentConfig := configUtils.GetComponentEnvironmentConfig(ra, envName, componentName)

	componentState, err := deployments.GetComponentStateFromSpec(eh.client, appName, deployment, rd.Status, environmentConfig, *componentToPatch)
	if err != nil {
		return err
	}

	if !strings.EqualFold(componentState.Status, deploymentModels.ConsistentComponent.String()) {
		return environmentModels.CannotRestartComponent(appName, componentName, componentState.Status)
	}

	err = eh.patchRdForRestart(appName, rd, index, componentToPatch)
	if err != nil {
		return err
	}

	return nil
}

func getReplicasForComponentInEnvironment(environmentConfig *v1.RadixEnvironmentConfig) (*int, error) {
	if environmentConfig != nil {
		return environmentConfig.Replicas, nil
	}

	return nil, nil
}

func (eh EnvironmentHandler) patchRdForRestart(appName string,
	rd *v1.RadixDeployment, componentIndex int, componentToPatch *v1.RadixDeployComponent) error {

	oldJSON, err := json.Marshal(rd)
	if err != nil {
		return err
	}

	environmentVariables := componentToPatch.EnvironmentVariables
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
