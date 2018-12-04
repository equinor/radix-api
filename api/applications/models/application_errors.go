package models

import (
	"fmt"

	"github.com/statoil/radix-api/api/utils"
)

// AppNameAndBranchAreRequiredForStartingPipeline Cannot start pipeline when appname and branch are missing
func AppNameAndBranchAreRequiredForStartingPipeline() error {
	return utils.ValidationError("Radix Application Pipeline", "App name and branch are required")
}

// UnmatchedBranchToEnvironment Triggering a pipeline on a un-mapped branch is not allowed
func UnmatchedBranchToEnvironment(branch string) error {
	return utils.ValidationError("Radix Application Pipeline", fmt.Sprintf("Failed to match environment to branch: %s", branch))
}
