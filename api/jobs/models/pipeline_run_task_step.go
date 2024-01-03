package models

import "time"

// PipelineRunTaskStep holds general information about pipeline run task steps
// swagger:model PipelineRunTaskStep
type PipelineRunTaskStep struct {
	// Name of the step
	//
	// required: true
	// example: build
	Name string `json:"name"`

	// Status of the task
	//
	// required: false
	// example: Completed
	Status string `json:"status"`

	// StatusMessage of the task
	//
	// required: false
	StatusMessage string `json:"statusMessage"`

	// Started timestamp
	//
	// required: false
	// swagger:strfmt date-time
	// example: 2006-01-02T15:04:05Z
	Started *time.Time `json:"started"`

	// Ended timestamp
	//
	// required: false
	// swagger:strfmt date-time
	// example: 2006-01-02T15:04:05Z
	Ended *time.Time `json:"ended"`
}
