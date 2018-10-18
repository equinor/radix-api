package applications

// PipelineParameters describe branch to build
// swagger:model PipelineParameters
type PipelineParameters struct {
	// Branch the branch to build
	//
	// required: true
	// example: master
	Branch string `json:"branch"`
}
