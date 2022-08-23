package environments

import (
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

type radixDeployCommonComponentUpdater interface {
	getComponentToPatch() v1.RadixCommonDeployComponent
	setEnvironmentVariablesToComponent(envVars v1.EnvVarsMap)
	getComponentStatus() string
	getRd() *v1.RadixDeployment
}

type baseComponentUpdater struct {
	appName          string
	envName          string
	componentName    string
	componentIndex   int
	componentToPatch v1.RadixCommonDeployComponent
	rd               *v1.RadixDeployment
	componentState   *deploymentModels.Component
	eh               EnvironmentHandler
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
	updater.base.rd.Spec.Components[updater.base.componentIndex].SetEnvironmentVariables(envVars)
}

func (updater *radixDeployComponentUpdater) getComponentStatus() string {
	return updater.base.componentState.Status
}

func (updater *radixDeployComponentUpdater) getRd() *v1.RadixDeployment {
	return updater.base.rd
}

func (updater *radixDeployJobComponentUpdater) getComponentToPatch() v1.RadixCommonDeployComponent {
	return updater.base.componentToPatch
}

func (updater *radixDeployJobComponentUpdater) setEnvironmentVariablesToComponent(envVars v1.EnvVarsMap) {
	updater.base.rd.Spec.Jobs[updater.base.componentIndex].SetEnvironmentVariables(envVars)
}

func (updater *radixDeployJobComponentUpdater) getComponentStatus() string {
	return updater.base.componentState.Status
}

func (updater *radixDeployJobComponentUpdater) getRd() *v1.RadixDeployment {
	return updater.base.rd
}
