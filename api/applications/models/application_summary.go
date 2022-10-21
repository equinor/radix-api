package models

import (
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	jobModels "github.com/equinor/radix-api/api/jobs/models"
)

// ApplicationSummary describe an application
// swagger:model ApplicationSummary
type ApplicationSummary struct {
	// Name the name of the application
	//
	// required: false
	// example: radix-canary-golang
	Name string `json:"name"`

	// LatestJob The latest started job
	//
	// required: false
	LatestJob *jobModels.JobSummary `json:"latestJob,omitempty"`

	// ActiveDeploymentComponents All component summaries of the active deployments
	//
	// required: false
	ActiveDeploymentComponents []*deploymentModels.Component `json:"activeDeploymentComponents,omitempty"`
}
