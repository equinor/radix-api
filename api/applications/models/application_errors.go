package models

import (
	"github.com/statoil/radix-api/api/utils"
)

// AppNameAndBranchAreRequiredForStartingPipeline Cannot start pipeline when appname and branch are missing
func AppNameAndBranchAreRequiredForStartingPipeline() error {
	return utils.ValidationError("Radix Application Pipeline", "App name and branch are required")
}
