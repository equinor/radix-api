package models

import (
	radixhttp "github.com/equinor/radix-common/net/http"
)

// AppNameAndBranchAreRequiredForStartingPipeline Cannot start pipeline when appname and branch are missing
func AppNameAndBranchAreRequiredForStartingPipeline() error {
	return radixhttp.ValidationError("Radix Application Pipeline", "App name and branch are required")
}

// OnePartOfDeployKeyIsNotAllowed Error message
func OnePartOfDeployKeyIsNotAllowed() error {
	return radixhttp.ValidationError("Radix Registration", "Setting public key, but no private key is not valid")
}
