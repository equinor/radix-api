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
	// - Pending = Environment exists in Radix config, but not in cluster
	// - Consistent = Environment exists in Radix config and in cluster
	// - Orphan = Environment does not exist in Radix config, but exists in cluster
	//
	// required: false
	// Enum: Pending,Consistent,Orphan
	// example: Consistent
	Status string `json:"status"`

	// ActiveDeployment The latest deployment in the environment
	//
	// required: false
	ActiveDeployment *deployModels.DeploymentSummary `json:"activeDeployment,omitempty"`

	// BranchMapping The branch mapped to this environment
	//
	// required: false
	BranchMapping string `json:"branchMapping,omitempty"`
}
