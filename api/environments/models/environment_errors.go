package models

import (
	"fmt"
	"strings"

	"github.com/equinor/radix-api/api/utils"
)

// NonExistingEnvironment No application found by name
func NonExistingEnvironment(underlyingError error, appName, envName string) error {
	return utils.TypeMissingError(fmt.Sprintf("Unable to get environment %s for app %s", envName, appName), underlyingError)
}

// CannotDeleteNonOrphanedEnvironment Can only delete orhaned environments
func CannotDeleteNonOrphanedEnvironment(appName, envName string) error {
	return utils.ValidationError("Radix Application Environment", fmt.Sprintf("Cannot delete non-orphaned environment %s for application %s", envName, appName))
}

// NonExistingComponent No component found by name
func NonExistingComponent(appName, componentName string) error {
	return utils.TypeMissingError(fmt.Sprintf("Unable to get component %s for app %s", componentName, appName), nil)
}

// CannotStopComponent Component cannot be stopped
func CannotStopComponent(appName, componentName, state string) error {
	return utils.ValidationError("Radix Application Component", fmt.Sprintf("Component %s for app %s cannot be stopped when in %s state", componentName, appName, strings.ToLower(state)))
}

// CannotStartComponent Component cannot be started
func CannotStartComponent(appName, componentName, state string) error {
	return utils.ValidationError("Radix Application Component", fmt.Sprintf("Component %s for app %s cannot be started when in %s state", componentName, appName, strings.ToLower(state)))
}

// CannotRestartComponent Component cannot be restarted
func CannotRestartComponent(appName, componentName, state string) error {
	return utils.ValidationError("Radix Application Component", fmt.Sprintf("Component %s for app %s cannot be restarted when in %s state", componentName, appName, strings.ToLower(state)))
}
