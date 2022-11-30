package environments

import (
	"context"
	"fmt"
	"strings"
	"time"

	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	environmentModels "github.com/equinor/radix-api/api/environments/models"
	"github.com/equinor/radix-api/api/utils/labelselector"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	jsonPatch "github.com/evanphx/json-patch"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
)

const restartedAtAnnotation = "radixapi/restartedAt"

// StopComponent Stops a component
func (eh EnvironmentHandler) StopComponent(appName, envName, componentName string) error {
	log.Infof("Stopping component %s, %s", componentName, appName)
	updater, err := eh.getRadixCommonComponentUpdater(appName, envName, componentName)
	if err != nil {
		return err
	}
	updater.setUserMutationTimestampAnnotation(radixutils.FormatTimestamp(time.Now()))
	if updater.getComponentToPatch().GetType() == v1.RadixComponentTypeJobScheduler {
		return environmentModels.JobComponentCanOnlyBeRestarted()
	}
	componentStatus := updater.getComponentStatus()
	if strings.EqualFold(componentStatus, deploymentModels.StoppedComponent.String()) {
		return environmentModels.CannotStopComponent(appName, componentName, componentStatus)
	}
	return eh.patchRadixDeploymentWithZeroReplicas(updater)
}

// StartComponent Starts a component
func (eh EnvironmentHandler) StartComponent(appName, envName, componentName string) error {
	log.Infof("Starting component %s, %s", componentName, appName)
	updater, err := eh.getRadixCommonComponentUpdater(appName, envName, componentName)
	if err != nil {
		return err
	}
	updater.setUserMutationTimestampAnnotation(radixutils.FormatTimestamp(time.Now()))
	if updater.getComponentToPatch().GetType() == v1.RadixComponentTypeJobScheduler {
		return environmentModels.JobComponentCanOnlyBeRestarted()
	}
	componentStatus := updater.getComponentStatus()
	if !strings.EqualFold(componentStatus, deploymentModels.StoppedComponent.String()) {
		return environmentModels.CannotStartComponent(appName, componentName, componentStatus)
	}
	updater.setUserMutationTimestampAnnotation(radixutils.FormatTimestamp(time.Now()))
	return eh.patchRadixDeploymentWithReplicasFromConfig(updater)
}

// RestartComponent Restarts a component
func (eh EnvironmentHandler) RestartComponent(appName, envName, componentName string) error {
	log.Infof("Restarting component %s, %s", componentName, appName)
	updater, err := eh.getRadixCommonComponentUpdater(appName, envName, componentName)
	if err != nil {
		return err
	}
	updater.setUserMutationTimestampAnnotation(radixutils.FormatTimestamp(time.Now()))
	componentStatus := updater.getComponentStatus()
	if !strings.EqualFold(componentStatus, deploymentModels.ConsistentComponent.String()) {
		return environmentModels.CannotRestartComponent(appName, componentName, componentStatus)
	}
	updater.setUserMutationTimestampAnnotation(radixutils.FormatTimestamp(time.Now()))
	return eh.patchRadixDeploymentWithTimestampInEnvVar(updater, defaults.RadixRestartEnvironmentVariable)
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

	// Get Kubernetes deployment object for auxiliary resource
	selector := labelselector.ForAuxiliaryResource(appName, componentName, auxType).String()
	envNs := operatorUtils.GetEnvironmentNamespace(appName, envName)
	deploymentList, err := eh.client.AppsV1().Deployments(envNs).List(context.TODO(), metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return err
	}
	// Return error if deployment object not found
	if len(deploymentList.Items) == 0 {
		return environmentModels.MissingAuxiliaryResourceDeployment(appName, componentName)
	}

	if !canDeploymentBeRestarted(&deploymentList.Items[0]) {
		return environmentModels.CannotRestartAuxiliaryResource(appName, componentName)
	}
	updater, err := eh.getRadixCommonComponentUpdater(appName, envName, componentName)
	if err != nil {
		return err
	}
	updater.setUserMutationTimestampAnnotation(radixutils.FormatTimestamp(time.Now()))
	return eh.patchDeploymentForRestart(&deploymentList.Items[0])
}

func canDeploymentBeRestarted(deployment *appsv1.Deployment) bool {
	if deployment == nil {
		return false
	}

	return deploymentModels.ComponentStatusFromDeployment(deployment) == deploymentModels.ConsistentComponent
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

func getReplicasForComponentInEnvironment(environmentConfig v1.RadixCommonEnvironmentConfig) (*int, error) {
	if environmentConfig != nil {
		return environmentConfig.GetReplicas(), nil
	}

	return nil, nil
}

func (eh EnvironmentHandler) patch(namespace, name string, oldJSON, newJSON []byte) error {
	patchBytes, err := jsonPatch.CreateMergePatch(oldJSON, newJSON)

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
