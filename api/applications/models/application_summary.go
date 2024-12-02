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
	// required: true
	// example: radix-canary-golang
	Name string `json:"name"`

	// LatestJob The latest started job
	//
	// required: false
	LatestJob *jobModels.JobSummary `json:"latestJob,omitempty"`

	// EnvironmentActiveComponents All component summaries of the active deployments in the environments
	//
	// required: false
	EnvironmentActiveComponents map[string][]*deploymentModels.Component `json:"environmentActiveComponents,omitempty"`
}
