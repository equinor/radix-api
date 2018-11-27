package models

import (
	jobModels "github.com/statoil/radix-api/api/jobs/models"
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
}
