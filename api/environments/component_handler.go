package environments

import (
	"fmt"
	"strings"
	"time"

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

	ra, _ := eh.radixclient.RadixV1().RadixApplications(crdUtils.GetAppNamespace(appName)).Get(appName, metav1.GetOptions{})
	replicas := getReplicasForComponentInEnvironment(ra, envName, componentName)

	err = eh.patchReplicasOnRD(rd, componentName, replicas)
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

	err = eh.updateRestartEnvironmentVariableOnRD(rd, componentName)
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

func (eh EnvironmentHandler) updateRestartEnvironmentVariableOnRD(rd *v1.RadixDeployment, componentName string) error {
	for index, component := range rd.Spec.Components {
		if strings.EqualFold(component.Name, componentName) {
			environmentVariables := component.EnvironmentVariables
			if environmentVariables == nil {
				environmentVariables = make(map[string]string)
			}

			environmentVariables[defaults.RadixRestartEnvironmentVariable] = utils.FormatTimestamp(time.Now())
			rd.Spec.Components[index].EnvironmentVariables = environmentVariables
			break
		}
	}

	_, err := eh.radixclient.RadixV1().RadixDeployments(rd.GetNamespace()).Update(rd)
	if err != nil {
		return fmt.Errorf("Failed to update deployment object: %v", err)
	}

	return nil
}

func (eh EnvironmentHandler) patchReplicasOnRD(rd *v1.RadixDeployment, component string, replicas *int) error {
	var patchBytes []byte

	for index := range rd.Spec.Components {
		if strings.EqualFold(rd.Spec.Components[index].Name, component) {
			newReplica := 1

			if replicas != nil {
				newReplica = *replicas
			}

			patchJSON := fmt.Sprintf(`[{"op": "replace", "path": "/spec/components/%d/replicas","value": %d}]`, index, newReplica)
			patchBytes = []byte(patchJSON)
			break
		}
	}

	if patchBytes != nil {
		_, err := eh.radixclient.RadixV1().RadixDeployments(rd.GetNamespace()).Patch(rd.GetName(), types.JSONPatchType, patchBytes)
		if err != nil {
			return fmt.Errorf("Failed to patch deployment object: %v", err)
		}
	}

	return nil
}
