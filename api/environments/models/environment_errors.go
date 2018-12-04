package models

import (
	"fmt"

	"github.com/statoil/radix-api/api/utils"
)

// NonExistingEnvironment No application found by name
func NonExistingEnvironment(underlyingError error, appName, envName string) error {
	return utils.TypeMissingError(fmt.Sprintf("Unable to get environment %s for app %s", envName, appName), underlyingError)
}
