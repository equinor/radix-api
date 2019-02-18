package models

import (
	"fmt"

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
