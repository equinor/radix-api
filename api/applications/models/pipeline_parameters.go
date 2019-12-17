package models

// PipelineParametersPromote identify deployment to promote and a target environment
// swagger:model PipelineParametersPromote
type PipelineParametersPromote struct {
	// ID of the deployment to promote
	// REQUIRED for "promote" pipeline
	//
	// example: dev-9tyu1-tftmnqzq
	DeploymentName string `json:"deploymentName"`

	// Name of environment where to look for the deployment to be promoted
	// REQUIRED for "promote" pipeline
	//
	// example: prod
	FromEnvironment string `json:"fromEnvironment"`

	// Name of environment to receive the promoted deployment
	// REQUIRED for "promote" pipeline
	//
	// example: prod
	ToEnvironment string `json:"toEnvironment"`
}

// PipelineParametersBuild describe branch to build and its commit ID
// swagger:model PipelineParametersBuild
type PipelineParametersBuild struct {
	// Branch the branch to build
	// REQUIRED for "build" and "build-deploy" pipelines
	//
	// example: master
	Branch string `json:"branch"`

	// CommitID the commit ID of the branch to build
	// REQUIRED for "build" and "build-deploy" pipelines
	//
	// example: 4faca8595c5283a9d0f17a623b9255a0d9866a2e
	CommitID string `json:"commitID"`

	// PushImage should image be pushed to container registry. Defaults pushing
	//
	// example: true
	PushImage string `json:"pushImage"`
}

// PushImageToContainerRegistry Normalises the "PushImage" param from a string
func (pipeParam PipelineParametersBuild) PushImageToContainerRegistry() bool {
	return !(pipeParam.PushImage == "0" || pipeParam.PushImage == "false")
}
