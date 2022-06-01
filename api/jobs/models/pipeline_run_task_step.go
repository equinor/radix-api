package models

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
	// Enum: Waiting,Running,Succeeded,Failed
	// example: Waiting
	Status string `json:"status"`

	// StatusMessage of the task
	//
	// required: false
	StatusMessage string `json:"statusMessage"`

	// Started timestamp
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	Started string `json:"started"`

	// Ended timestamp
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	Ended string `json:"ended"`
}
