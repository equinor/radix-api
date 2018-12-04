package models

import (
	deployModels "github.com/statoil/radix-api/api/deployments/models"
)

// Environment holds detail information about environment
// swagger:model Environment
type Environment struct {
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

	// Deployments All deployments in environment
	//
	// required: false
	Deployments []*deployModels.DeploymentSummary `json:"deployments,omitempty"`

	// Secrets All secrets in environment
	//
	// required: false
	Secrets []Secret `json:"secrets,omitempty"`

	// ActiveDeployment The latest deployment in the environment
	//
	// required: false
	ActiveDeployment *deployModels.Deployment `json:"activeDeployment,omitempty"`

	// BranchMapping The branch mapped to this environment
	//
	// required: false
	// example: master
	BranchMapping string `json:"branchMapping,omitempty"`
}
