package models

import (
	environmentModels "github.com/equinor/radix-api/api/environments/models"
	jobModels "github.com/equinor/radix-api/api/jobs/models"
)

// Application details of an application
// swagger:model Application
type Application struct {
	// Name the name of the application
	//
	// required: false
	// example: radix-canary-golang
	Name string `json:"name"`

	// Owner of the application (email). Can be a single person or a shared group email
	//
	// required: false
	Owner string `json:"owner"`

	// Creator of the application (user principle name).
	//
	// required: false
	Creator string `json:"creator"`

	// Registration registration details
	//
	// required: false
	Registration ApplicationRegistration `json:"registration"`

	// Environments List of environments for this application
	//
	// required: false
	Environments []*environmentModels.EnvironmentSummary `json:"environments"`

	// Jobs list of run jobs for the application
	//
	// required: false
	Jobs []*jobModels.JobSummary `json:"jobs"`

	// App alias showing nicer endpoint for application
	//
	// required: false
	AppAlias *ApplicationAlias `json:"appAlias,omitempty"`
}
