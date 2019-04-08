package models

import (
	"fmt"

	"github.com/equinor/radix-api/api/utils"
)

// AppNameAndBranchAreRequiredForStartingPipeline Cannot start pipeline when appname and branch are missing
func AppNameAndBranchAreRequiredForStartingPipeline() error {
	return utils.ValidationError("Radix Application Pipeline", "App name and branch are required")
}

// UnmatchedBranchToEnvironment Triggering a pipeline on a un-mapped branch is not allowed
func UnmatchedBranchToEnvironment(branch string) error {
	return utils.ValidationError("Radix Application Pipeline", fmt.Sprintf("Failed to match environment to branch: %s", branch))
}

// OnePartOfDeployKeyIsNotAllowed Error message
func OnePartOfDeployKeyIsNotAllowed() error {
	return utils.ValidationError("Radix Registration", fmt.Sprintf("Setting public key, but no private key is not valid"))
}
