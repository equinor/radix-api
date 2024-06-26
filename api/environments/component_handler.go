package environments

import (
	"context"
	"strings"
	"time"

	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	environmentModels "github.com/equinor/radix-api/api/environments/models"
	"github.com/equinor/radix-api/api/utils/labelselector"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	jsonPatch "github.com/evanphx/json-patch/v5"
	"github.com/rs/zerolog/log"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
)

const (
	restartedAtAnnotation = "radixapi/restartedAt"
	maxScaleReplicas      = 20
)

// StopComponent Stops a component
func (eh EnvironmentHandler) StopComponent(ctx context.Context, appName, envName, componentName string, ignoreComponentStatusError bool) error {

	log.Ctx(ctx).Info().Msgf("Stopping component %s, %s", componentName, appName)
	updater, err := eh.getRadixCommonComponentUpdater(ctx, appName, envName, componentName)
	if err != nil {
		return err
	}
	if updater.getComponentToPatch().GetType() == v1.RadixComponentTypeJob {
		return environmentModels.JobComponentCanOnlyBeRestarted()
	}
	componentStatus := updater.getComponentStatus()
	if strings.EqualFold(componentStatus, deploymentModels.StoppedComponent.String()) {
		if ignoreComponentStatusError {
			return nil
		}
		return environmentModels.CannotStopComponent(appName, componentName, componentStatus)
	}
	return eh.patchRadixDeploymentWithZeroReplicas(ctx, updater)
}

// StartComponent Starts a component
func (eh EnvironmentHandler) StartComponent(ctx context.Context, appName, envName, componentName string, ignoreComponentStatusError bool) error {
	log.Ctx(ctx).Info().Msgf("Starting component %s, %s", componentName, appName)
	updater, err := eh.getRadixCommonComponentUpdater(ctx, appName, envName, componentName)
	if err != nil {
		return err
	}
	if updater.getComponentToPatch().GetType() == v1.RadixComponentTypeJob {
		return environmentModels.JobComponentCanOnlyBeRestarted()
	}
	componentStatus := updater.getComponentStatus()
	if !strings.EqualFold(componentStatus, deploymentModels.StoppedComponent.String()) {
		if ignoreComponentStatusError {
			return nil
		}
		return environmentModels.CannotStartComponent(appName, componentName, componentStatus)
	}
	return eh.patchRadixDeploymentWithReplicasFromConfig(ctx, updater)
}

// RestartComponent Restarts a component
func (eh EnvironmentHandler) RestartComponent(ctx context.Context, appName, envName, componentName string, ignoreComponentStatusError bool) error {
	log.Ctx(ctx).Info().Msgf("Restarting component %s, %s", componentName, appName)
	updater, err := eh.getRadixCommonComponentUpdater(ctx, appName, envName, componentName)
	if err != nil {
		return err
	}
	componentStatus := updater.getComponentStatus()
	if !strings.EqualFold(componentStatus, deploymentModels.ConsistentComponent.String()) {
		if ignoreComponentStatusError {
			return nil
		}
		return environmentModels.CannotRestartComponent(appName, componentName, componentStatus)
	}
	return eh.patchRadixDeploymentWithTimestampInEnvVar(ctx, updater, defaults.RadixRestartEnvironmentVariable)
}

// RestartComponentAuxiliaryResource Restarts a component's auxiliary resource
func (eh EnvironmentHandler) RestartComponentAuxiliaryResource(ctx context.Context, appName, envName, componentName, auxType string) error {
	log.Ctx(ctx).Info().Msgf("Restarting auxiliary resource %s for component %s, %s", auxType, componentName, appName)

	deploySummary, err := eh.deployHandler.GetLatestDeploymentForApplicationEnvironment(ctx, appName, envName)
	if err != nil {
		return err
	}

	componentsDto, err := eh.deployHandler.GetComponentsForDeployment(ctx, appName, deploySummary.Name, envName)
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
	deploymentList, err := eh.accounts.UserAccount.Client.AppsV1().Deployments(envNs).List(ctx, metav1.ListOptions{LabelSelector: selector})
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

	return eh.patchDeploymentForRestart(ctx, &deploymentList.Items[0])
}

// ScaleComponent Scale a component replicas
func (eh EnvironmentHandler) ScaleComponent(ctx context.Context, appName, envName, componentName string, replicas int) error {
	if replicas < 0 {
		return environmentModels.CannotScaleComponentToNegativeReplicas(appName, envName, componentName)
	}
	if replicas > maxScaleReplicas {
		return environmentModels.CannotScaleComponentToMoreThanMaxReplicas(appName, envName, componentName, maxScaleReplicas)
	}
	log.Ctx(ctx).Info().Msgf("Scaling component %s, %s to %d replicas", componentName, appName, replicas)
	updater, err := eh.getRadixCommonComponentUpdater(ctx, appName, envName, componentName)
	if err != nil {
		return err
	}
	componentStatus := updater.getComponentStatus()
	if !radixutils.ContainsString(validaStatusesToScaleComponent, componentStatus) {
		return environmentModels.CannotScaleComponent(appName, envName, componentName, componentStatus)
	}
	return eh.patchRadixDeploymentWithReplicas(ctx, updater, replicas)
}

func canDeploymentBeRestarted(deployment *appsv1.Deployment) bool {
	if deployment == nil {
		return false
	}

	return deploymentModels.ComponentStatusFromDeployment(deployment) == deploymentModels.ConsistentComponent
}

func (eh EnvironmentHandler) patchDeploymentForRestart(ctx context.Context, deployment *appsv1.Deployment) error {
	deployClient := eh.accounts.UserAccount.Client.AppsV1().Deployments(deployment.GetNamespace())

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		deployToPatch, err := deployClient.Get(ctx, deployment.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}
		if deployToPatch.Spec.Template.Annotations == nil {
			deployToPatch.Spec.Template.Annotations = make(map[string]string)
		}

		deployToPatch.Spec.Template.Annotations[restartedAtAnnotation] = radixutils.FormatTimestamp(time.Now())
		_, err = deployClient.Update(ctx, deployToPatch, metav1.UpdateOptions{})
		return err
	})
}

func getReplicasForComponentInEnvironment(environmentConfig v1.RadixCommonEnvironmentConfig) (*int, error) {
	if environmentConfig != nil {
		return environmentConfig.GetReplicas(), nil
	}

	return nil, nil
}

func (eh EnvironmentHandler) patch(ctx context.Context, namespace, name string, oldJSON, newJSON []byte) error {
	patchBytes, err := jsonPatch.CreateMergePatch(oldJSON, newJSON)
	if err != nil {
		return err
	}

	if patchBytes != nil {
		_, err := eh.accounts.UserAccount.RadixClient.RadixV1().RadixDeployments(namespace).Patch(ctx, name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func (eh EnvironmentHandler) patchRadixDeploymentWithReplicasFromConfig(ctx context.Context, updater radixDeployCommonComponentUpdater) error {
	return eh.commit(ctx, updater, func(updater radixDeployCommonComponentUpdater) error {
		newReplica := 1
		replicas, err := getReplicasForComponentInEnvironment(updater.getEnvironmentConfig())
		if err != nil {
			return err
		}
		if replicas != nil {
			newReplica = *replicas
		}
		updater.setReplicasToComponent(&newReplica)
		updater.setUserMutationTimestampAnnotation(radixutils.FormatTimestamp(time.Now()))
		return nil
	})
}

func (eh EnvironmentHandler) patchRadixDeploymentWithReplicas(ctx context.Context, updater radixDeployCommonComponentUpdater, replicas int) error {
	return eh.commit(ctx, updater, func(updater radixDeployCommonComponentUpdater) error {
		updater.setReplicasToComponent(&replicas)
		updater.setUserMutationTimestampAnnotation(radixutils.FormatTimestamp(time.Now()))
		return nil
	})
}

func (eh EnvironmentHandler) patchRadixDeploymentWithTimestampInEnvVar(ctx context.Context, updater radixDeployCommonComponentUpdater, envVarName string) error {
	return eh.commit(ctx, updater, func(updater radixDeployCommonComponentUpdater) error {
		environmentVariables := updater.getComponentToPatch().GetEnvironmentVariables()
		if environmentVariables == nil {
			environmentVariables = make(map[string]string)
		}
		environmentVariables[envVarName] = radixutils.FormatTimestamp(time.Now())
		updater.setEnvironmentVariablesToComponent(environmentVariables)
		updater.setUserMutationTimestampAnnotation(radixutils.FormatTimestamp(time.Now()))
		return nil
	})
}

func (eh EnvironmentHandler) patchRadixDeploymentWithZeroReplicas(ctx context.Context, updater radixDeployCommonComponentUpdater) error {
	return eh.commit(ctx, updater, func(updater radixDeployCommonComponentUpdater) error {
		newReplica := 0
		updater.setReplicasToComponent(&newReplica)
		updater.setUserMutationTimestampAnnotation(radixutils.FormatTimestamp(time.Now()))
		return nil
	})
}
