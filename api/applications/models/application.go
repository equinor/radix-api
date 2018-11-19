package models

import (
	jobModels "github.com/statoil/radix-api/api/jobs/models"
)

// Application details of an application
// swagger:model Application
type Application struct {
	// Name the name of the application
	//
	// required: false
	// example: radix-canary-golang
	Name string `json:"name"`

	// Registration registration details
	//
	// required: false
	Registration ApplicationRegistration `json:"registration"`

	// Jobs list of run jobs for the application
	//
	// required: false
	Jobs []jobModels.JobSummary `json:"jobs"`
}
