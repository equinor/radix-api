package models

import (
	deployModels "github.com/statoil/radix-api/api/deployments/models"
)

// EnvironmentSummary holds general information about environment
// swagger:model EnvironmentSummary
type EnvironmentSummary struct {
	// Name of the environment
	//
	// required: false
	// example: prod
	Name string `json:"name"`

	// Status of the environment
	//
	// required: false
	// example: Consistent
	Status ConfigurationStatus `json:"status"`

	// ActiveDeployment The latest deployment in the environment
	//
	// required: false
	// example: master
	ActiveDeployment deployModels.DeploymentSummary `json:"activeDeployment"`

	// BranchMapping The branch mapped to this environment
	//
	// required: false
	// example: master
	BranchMapping string `json:"branchMapping"`
}
