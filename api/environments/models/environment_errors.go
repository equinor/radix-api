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
