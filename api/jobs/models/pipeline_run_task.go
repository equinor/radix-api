package models

// PipelineRunTask holds general information about pipeline run task
// swagger:model PipelineRunTask
type PipelineRunTask struct {
	// Name of the task
	//
	// required: true
	// example: build
	Name string `json:"name"`

	// RealName Name of the pipeline run in the namespace
	//
	// required: true
	// example: radix-tekton-task-dev-2022-05-09-abcde
	RealName string `json:"realName"`

	// PipelineRunEnv Environment of the pipeline run
	//
	// required: true
	// example: prod
	PipelineRunEnv string `json:"pipelineRunEnv"`

	// PipelineName of the task
	//
	// required: true
	// example: build-pipeline
	PipelineName string `json:"pipelineName"`

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
