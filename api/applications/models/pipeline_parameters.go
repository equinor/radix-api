package models

// PipelineParameters describe branch to build and its commit ID
// swagger:model PipelineParameters
type PipelineParameters struct {
	// Branch the branch to build
	//
	// required: true
	// example: master
	Branch string `json:"branch"`

	// CommitID the commit ID of the branch to build
	//
	// required: true
	// example: 4faca8595c5283a9d0f17a623b9255a0d9866a2e
	CommitID string `json:"commitID"`

	// PushImage should image be pushed to container registry. Defaults pushing
	//
	// required: false
	// example: true
	PushImage string `json:"pushImage"`
}

func (pipeParam PipelineParameters) PushImageToContainerRegistry() bool {
	return !(pipeParam.PushImage == "0" || pipeParam.PushImage == "false")
}
