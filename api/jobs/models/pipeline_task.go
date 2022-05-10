package models

// PipelineTask holds general information about pipeline task
// swagger:model PipelineTask
type PipelineTask struct {
	// Name of the task
	//
	// required: false
	// example: build
	Name string `json:"name"`

	// RealName Name of the pipeline-run in the namespace
	//
	// required: false
	// example: radix-tekton-task-dev-2022-05-09-abcde
	RealName string `json:"realName"`

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

	// Pod name
	//
	// required: false
	PodName string `json:"podName"`
}
