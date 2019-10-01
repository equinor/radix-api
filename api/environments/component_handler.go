package environments

import (
	"encoding/json"
	"fmt"
	"strings"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// StopComponent Stops a component
func (eh EnvironmentHandler) StopComponent(appName, envName, componentName string) error {
	log.Infof("Stopping component %s, %s", componentName, appName)
	envNs := crdUtils.GetEnvironmentNamespace(appName, envName)
	deployment, err := eh.deployHandler.GetLatestDeploymentForApplicationEnvironment(appName, envName)
	if err != nil {
		return err
	}

	rd, err := eh.radixclient.RadixV1().RadixDeployments(envNs).Get(deployment.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	noReplicas := 0
	err = eh.patchReplicasOnRD(rd, componentName, &noReplicas)
	if err != nil {
		return err
	}

	return nil
}

// StartComponent Starts a component
func (eh EnvironmentHandler) StartComponent(appName, envName, componentName string) error {
	log.Infof("Starting component %s, %s", componentName, appName)
	envNs := crdUtils.GetEnvironmentNamespace(appName, envName)
	deployment, err := eh.deployHandler.GetLatestDeploymentForApplicationEnvironment(appName, envName)
	if err != nil {
		return err
	}

	rd, err := eh.radixclient.RadixV1().RadixDeployments(envNs).Get(deployment.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	ra, _ := eh.radixclient.RadixV1().RadixApplications(utils.GetAppNamespace(appName)).Get(appName, metav1.GetOptions{})
	replicas := getReplicasForComponentInEnvironment(ra, envName, componentName)

	err = eh.patchReplicasOnRD(rd, componentName, replicas)
	if err != nil {
		return err
	}

	return nil
}

// RestartComponent Restarts a component
func (eh EnvironmentHandler) RestartComponent(appName, envName, componentName string) error {
	err := eh.StopComponent(appName, envName, componentName)
	if err != nil {
		return err
	}

	err = eh.StartComponent(appName, envName, componentName)
	if err != nil {
		return err
	}

	return nil
}

func getReplicasForComponentInEnvironment(ra *v1.RadixApplication, envName, componentName string) *int {
	environmentConfig := getEnvironment(getComponent(ra, componentName), envName)
	if environmentConfig != nil {
		return environmentConfig.Replicas
	}

	return nil
}

func getComponent(ra *v1.RadixApplication, componentName string) *v1.RadixComponent {
	for _, component := range ra.Spec.Components {
		if strings.EqualFold(component.Name, componentName) {
			return &component
		}
	}

	return nil
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

func (eh EnvironmentHandler) patchReplicasOnRD(rd *v1.RadixDeployment, component string, replicas *int) error {
	newRD := rd.DeepCopy()
	for index := range newRD.Spec.Components {
		if strings.EqualFold(newRD.Spec.Components[index].Name, component) {
			newRD.Spec.Components[index].Replicas = replicas
			break
		}
	}

	rdJSON, err := json.Marshal(rd)
	if err != nil {
		return fmt.Errorf("Failed to marshal old deployment object: %v", err)
	}

	newRDJSON, err := json.Marshal(newRD)
	if err != nil {
		return fmt.Errorf("Failed to marshal new deployment object: %v", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(rdJSON, newRDJSON, v1.RadixDeployment{})
	if err != nil {
		return fmt.Errorf("Failed to create two way merge patch deployment objects: %v", err)
	}

	if !isEmptyPatch(patchBytes) {
		_, err := eh.radixclient.RadixV1().RadixDeployments(rd.GetNamespace()).Patch(rd.GetName(), types.StrategicMergePatchType, patchBytes)
		if err != nil {
			return fmt.Errorf("Failed to patch deployment object: %v", err)
		}
	}

	return nil
}

func isEmptyPatch(patchBytes []byte) bool {
	return string(patchBytes) == "{}"
}
