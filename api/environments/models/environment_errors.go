package models

import (
	"fmt"
	"strings"

	radixhttp "github.com/equinor/radix-common/net/http"
)

// NonExistingEnvironment No application found by name
func NonExistingEnvironment(underlyingError error, appName, envName string) error {
	return radixhttp.TypeMissingError(fmt.Sprintf("Unable to get environment %s for app %s", envName, appName), underlyingError)
}

// CannotDeleteNonOrphanedEnvironment Can only delete orphaned environments
func CannotDeleteNonOrphanedEnvironment(appName, envName string) error {
	return radixhttp.ValidationError("Radix Application Environment", fmt.Sprintf("Cannot delete non-orphaned environment %s for application %s", envName, appName))
}

// NonExistingComponent No component found by name
func NonExistingComponent(appName, componentName string) error {
	return radixhttp.TypeMissingError(fmt.Sprintf("Unable to get component %s for app %s", componentName, appName), nil)
}

// NonExistingComponentAuxiliaryType Auxiliary resource for component component not found
func NonExistingComponentAuxiliaryType(appName, componentName, auxType string) error {
	return radixhttp.TypeMissingError(fmt.Sprintf("%s resource does not exist for component %s in app %s", auxType, componentName, appName), nil)
}

// CannotStopComponent Component cannot be stopped
func CannotStopComponent(appName, componentName, state string) error {
	return radixhttp.ValidationError("Radix Application Component", fmt.Sprintf("Component %s for app %s cannot be stopped when in %s state", componentName, appName, strings.ToLower(state)))
}

// CannotStartComponent Component cannot be started
func CannotStartComponent(appName, componentName, state string) error {
	return radixhttp.ValidationError("Radix Application Component", fmt.Sprintf("Component %s for app %s cannot be started when in %s state", componentName, appName, strings.ToLower(state)))
}

// CannotRestartComponent Component cannot be restarted
func CannotRestartComponent(appName, componentName, state string) error {
	return radixhttp.ValidationError("Radix Application Component", fmt.Sprintf("Component %s for app %s cannot be restarted when in %s state", componentName, appName, strings.ToLower(state)))
}

// CannotRestartAuxiliaryResource Auxiliary resource cannot be restarted
func CannotRestartAuxiliaryResource(appName, componentName string) error {
	return radixhttp.ValidationError("Radix Application Auxiliary Resource", fmt.Sprintf("Auxiliary resource for component %s for app %s cannot be restarted", componentName, appName))
}

// MissingAuxiliaryResourceDeployment Auxiliary resource cannot be found
func MissingAuxiliaryResourceDeployment(appName, componentName string) error {
	return radixhttp.UnexpectedError("Radix Application Auxiliary Resource", fmt.Errorf("deployment for auxiliary resource not found"))
}

// JobComponentCanOnlyBeRestarted Job component cannot be started or stopped, but only restarted
func JobComponentCanOnlyBeRestarted() error {
	return radixhttp.UnexpectedError("Radix Application Job Component", fmt.Errorf("job component can only be restarted"))
}
