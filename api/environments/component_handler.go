package environments

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/equinor/radix-api/api/deployments"
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	environmentModels "github.com/equinor/radix-api/api/environments/models"
	"github.com/equinor/radix-api/api/utils/labelselector"
	radixutils "github.com/equinor/radix-common/utils"
	configUtils "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	deployUtils "github.com/equinor/radix-operator/pkg/apis/deployment"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	jsonpatch "github.com/evanphx/json-patch"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
)

const restartedAtAnnotation = "radixapi/restartedAt"

// StopComponent Stops a component
func (eh EnvironmentHandler) StopComponent(appName, envName, componentName string) error {
	deploymentSummary, rd, err := eh.getRadixDeployment(appName, envName)
	if err != nil {
		return err
	}

	index, componentToPatch := deployUtils.GetDeploymentComponent(rd, componentName)
	if componentToPatch == nil {
		return environmentModels.NonExistingComponent(appName, componentName)
	}

	ra, _ := eh.getRadixApplicationInAppNamespace(appName)
	environmentConfig := configUtils.GetComponentEnvironmentConfig(ra, envName, componentName)

	componentState, err := deployments.GetComponentStateFromSpec(eh.client, appName, deploymentSummary, rd.Status, environmentConfig, componentToPatch)
	if err != nil {
		return err
	}

	if strings.EqualFold(componentState.Status, deploymentModels.StoppedComponent.String()) {
		return environmentModels.CannotStopComponent(appName, componentName, componentState.Status)
	}

	log.Infof("Stopping component %s, %s", componentName, appName)
	noReplicas := 0
	err = eh.patchReplicasOnRD(rd, index, &noReplicas)
	if err != nil {
		return err
	}

	return nil
}

// StartComponent Starts a component
func (eh EnvironmentHandler) StartComponent(appName, envName, componentName string) error {
	deploymentSummary, rd, err := eh.getRadixDeployment(appName, envName)
	if err != nil {
		return err
	}

	index, componentToPatch := deployUtils.GetDeploymentComponent(rd, componentName)
	if componentToPatch == nil {
		return environmentModels.NonExistingComponent(appName, componentName)
	}

	ra, _ := eh.getRadixApplicationInAppNamespace(appName)
	environmentConfig := configUtils.GetComponentEnvironmentConfig(ra, envName, componentName)

	replicas, err := getReplicasForComponentInEnvironment(environmentConfig)
	if err != nil {
		return err
	}

	componentState, err := deployments.GetComponentStateFromSpec(eh.client, appName, deploymentSummary, rd.Status, environmentConfig, componentToPatch)
	if err != nil {
		return err
	}

	if !strings.EqualFold(componentState.Status, deploymentModels.StoppedComponent.String()) {
		return environmentModels.CannotStartComponent(appName, componentName, componentState.Status)
	}

	log.Infof("Starting component %s, %s", componentName, appName)
	err = eh.patchReplicasOnRD(rd, index, replicas)
	if err != nil {
		return err
	}

	return nil
}

// RestartComponent Restarts a component
func (eh EnvironmentHandler) RestartComponent(appName, envName, componentName string) error {
	log.Infof("Restarting component %s, %s", componentName, appName)
	deploymentSummary, rd, err := eh.getRadixDeployment(appName, envName)
	if err != nil {
		return err
	}

	index, componentToPatch := deployUtils.GetDeploymentComponent(rd, componentName)
	if componentToPatch == nil {
		return environmentModels.NonExistingComponent(appName, componentName)
	}

	ra, _ := eh.getRadixApplicationInAppNamespace(appName)
	environmentConfig := configUtils.GetComponentEnvironmentConfig(ra, envName, componentName)

	componentState, err := deployments.GetComponentStateFromSpec(eh.client, appName, deploymentSummary, rd.Status, environmentConfig, componentToPatch)
	if err != nil {
		return err
	}

	if !strings.EqualFold(componentState.Status, deploymentModels.ConsistentComponent.String()) {
		return environmentModels.CannotRestartComponent(appName, componentName, componentState.Status)
	}

	err = eh.patchRdForRestart(rd, index, componentToPatch)
	if err != nil {
		return err
	}

	return nil
}

// RestartComponentAuxiliaryResource Restarts a component's auxiliary resource
func (eh EnvironmentHandler) RestartComponentAuxiliaryResource(appName, envName, componentName, auxType string) error {
	log.Infof("Restarting auxiliary resource %s for component %s, %s", auxType, componentName, appName)

	deploySummary, err := eh.deployHandler.GetLatestDeploymentForApplicationEnvironment(appName, envName)
	if err != nil {
		return err
	}

	componentsDto, err := eh.deployHandler.GetComponentsForDeployment(appName, deploySummary)
	if err != nil {
		return err
	}

	var componentDto *deploymentModels.Component
	for _, c := range componentsDto {
		if c.Name == componentName {
			componentDto = c
			break
		}
	}

	// Check if component exists
	if componentDto == nil {
		return environmentModels.NonExistingComponent(appName, componentName)
	}

	// Check if auxiliary resource exists for component
	auxResourceDto := componentDto.GetAuxiliaryResourceByType(auxType)
	if auxResourceDto == nil {
		return environmentModels.NonExistingComponentAuxiliaryType(appName, componentName, auxType)
	}

	// Check if auxiliary is in a state that allows it to be restarted
	if !canAuxiliaryResourceBeRestarted(auxResourceDto) {
		return environmentModels.CannotRestartAuxiliaryResource(appName, componentName)
	}

	// Get Kubernetes deployment object for auxiliary resource
	selector := labelselector.ForAuxiliaryResource(appName, componentName, auxType).String()
	envNs := operatorUtils.GetEnvironmentNamespace(appName, envName)
	deployments, err := eh.client.AppsV1().Deployments(envNs).List(context.TODO(), metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return err
	}
	// Return error if deployment object not found
	if len(deployments.Items) == 0 {
		return environmentModels.MissingAuxiliaryResourceDeployment(appName, componentName)
	}

	return eh.patchDeploymentForRestart(&deployments.Items[0])
}

func canAuxiliaryResourceBeRestarted(auxResource *deploymentModels.AuxiliaryResource) bool {
	if auxResource == nil {
		return false
	}

	isDeploymentRestartable := func(deploy *deploymentModels.AuxiliaryResourceDeployment) bool {
		return deploy != nil && deploy.Status == deploymentModels.ConsistentComponent.String()
	}

	switch auxResource.Type {
	case defaults.OAuthProxyAuxiliaryComponentType:
		if auxResource.OAuth2 == nil {
			return false
		}
		return isDeploymentRestartable(&auxResource.OAuth2.Deployment)
	default:
		return false
	}
}

func (eh EnvironmentHandler) patchDeploymentForRestart(deployment *appsv1.Deployment) error {
	deployClient := eh.client.AppsV1().Deployments(deployment.GetNamespace())

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		deployToPatch, err := deployClient.Get(context.TODO(), deployment.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}
		if deployToPatch.Spec.Template.Annotations == nil {
			deployToPatch.Spec.Template.Annotations = make(map[string]string)
		}

		deployToPatch.Spec.Template.Annotations[restartedAtAnnotation] = radixutils.FormatTimestamp(time.Now())
		_, err = deployClient.Update(context.TODO(), deployToPatch, metav1.UpdateOptions{})
		return err
	})
}

func getReplicasForComponentInEnvironment(environmentConfig *v1.RadixEnvironmentConfig) (*int, error) {
	if environmentConfig != nil {
		return environmentConfig.Replicas, nil
	}

	return nil, nil
}

func (eh EnvironmentHandler) patchRdForRestart(rd *v1.RadixDeployment, componentIndex int, componentToPatch *v1.RadixDeployComponent) error {

	oldJSON, err := json.Marshal(rd)
	if err != nil {
		return err
	}

	environmentVariables := componentToPatch.EnvironmentVariables
	if environmentVariables == nil {
		environmentVariables = make(map[string]string)
	}

	environmentVariables[defaults.RadixRestartEnvironmentVariable] = radixutils.FormatTimestamp(time.Now())
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

func (eh EnvironmentHandler) patchReplicasOnRD(rd *v1.RadixDeployment, componentIndex int, replicas *int) error {
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
		_, err := eh.radixclient.RadixV1().RadixDeployments(namespace).Patch(context.TODO(), name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
		if err != nil {
			return fmt.Errorf("failed to patch deployment object: %v", err)
		}
	}

	return nil
}
