package models

import (
	"fmt"

	radixhttp "github.com/equinor/radix-common/net/http"
)

// AppNameAndBranchAreRequiredForStartingPipeline Cannot start pipeline when appname and branch are missing
func AppNameAndBranchAreRequiredForStartingPipeline() error {
	return radixhttp.ValidationError("Radix Application Pipeline", "App name and branch are required")
}

// UnmatchedBranchToEnvironment Triggering a pipeline on a un-mapped branch is not allowed
func UnmatchedBranchToEnvironment(branch string) error {
	return radixhttp.ValidationError("Radix Application Pipeline", fmt.Sprintf("Failed to match environment to branch: %s", branch))
}

// OnePartOfDeployKeyIsNotAllowed Error message
func OnePartOfDeployKeyIsNotAllowed() error {
	return radixhttp.ValidationError("Radix Registration", "Setting public key, but no private key is not valid")
}
