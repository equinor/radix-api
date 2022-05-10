package models

// PipelineRun holds general information about pipeline-run
// swagger:model PipelineRun
type PipelineRun struct {
	// Name Original name of the pipeline-run
	//
	// required: false
	// example: build
	Name string `json:"name"`

	// RealName Name of the pipeline-run in the namespace
	//
	// required: false
	// example: radix-tekton-pipelinerun-dev-2022-05-09-abcde
	RealName string `json:"RealName"`

	// Status of the step
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

	// Tasks PipelineTask List of tasks
	//
	// required: false
	Tasks []PipelineTask `json:"tasks,omitempty"`
}
