package environments

import (
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

type radixDeployCommonComponentUpdater interface {
	getComponentToPatch() v1.RadixCommonDeployComponent
	setEnvironmentVariablesToComponent(envVars v1.EnvVarsMap)
	getComponentStatus() string
	getRadixDeployment() *v1.RadixDeployment
	getEnvironmentConfig() v1.RadixCommonEnvironmentConfig
	setReplicasToComponent(replicas *int)
	setUserMutationTimestampAnnotation(timestamp string)
}

type baseComponentUpdater struct {
	appName           string
	envName           string
	componentName     string
	componentIndex    int
	componentToPatch  v1.RadixCommonDeployComponent
	radixDeployment   *v1.RadixDeployment
	componentState    *deploymentModels.Component
	environmentConfig v1.RadixCommonEnvironmentConfig
}

type radixDeployComponentUpdater struct {
	base *baseComponentUpdater
}

type radixDeployJobComponentUpdater struct {
	base *baseComponentUpdater
}

func (updater *radixDeployComponentUpdater) getComponentToPatch() v1.RadixCommonDeployComponent {
	return updater.base.componentToPatch
}

func (updater *radixDeployComponentUpdater) setEnvironmentVariablesToComponent(envVars v1.EnvVarsMap) {
	updater.base.radixDeployment.Spec.Components[updater.base.componentIndex].SetEnvironmentVariables(envVars)
}

func (updater *radixDeployComponentUpdater) setReplicasToComponent(replicas *int) {
	updater.base.radixDeployment.Spec.Components[updater.base.componentIndex].Replicas = replicas
}

func (updater *radixDeployComponentUpdater) setUserMutationTimestampAnnotation(timestamp string) {
	updater.base.radixDeployment.Annotations["last-user-state-mutation-or-something"] = timestamp
}

func (updater *radixDeployComponentUpdater) getComponentStatus() string {
	return updater.base.componentState.Status
}

func (updater *radixDeployComponentUpdater) getRadixDeployment() *v1.RadixDeployment {
	return updater.base.radixDeployment
}

func (updater *radixDeployComponentUpdater) getEnvironmentConfig() v1.RadixCommonEnvironmentConfig {
	return updater.base.environmentConfig
}

func (updater *radixDeployJobComponentUpdater) getComponentToPatch() v1.RadixCommonDeployComponent {
	return updater.base.componentToPatch
}

func (updater *radixDeployJobComponentUpdater) setEnvironmentVariablesToComponent(envVars v1.EnvVarsMap) {
	updater.base.radixDeployment.Spec.Jobs[updater.base.componentIndex].SetEnvironmentVariables(envVars)
}

func (updater *radixDeployJobComponentUpdater) setUserMutationTimestampAnnotation(timestamp string) {
	updater.base.radixDeployment.Annotations["last-user-state-mutation-or-something"] = timestamp
}

func (updater *radixDeployJobComponentUpdater) setReplicasToComponent(replicas *int) {
	//job component has always 1 replica
}

func (updater *radixDeployJobComponentUpdater) getComponentStatus() string {
	return updater.base.componentState.Status
}

func (updater *radixDeployJobComponentUpdater) getRadixDeployment() *v1.RadixDeployment {
	return updater.base.radixDeployment
}

func (updater *radixDeployJobComponentUpdater) getEnvironmentConfig() v1.RadixCommonEnvironmentConfig {
	return updater.base.environmentConfig
}
