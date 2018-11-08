package models

import "fmt"

// Pipeline Enumeration of the different pipelines we support
type Pipeline int

const (
	// BuildDeploy Will do build based on docker file and deploy to mapped environment
	BuildDeploy Pipeline = iota

	// end marker of the enum
	numPipelines
)

func (p Pipeline) String() string {
	return [...]string{"build-deploy"}[p]
}

// GetPipelineFromName Gets pipeline from string
func GetPipelineFromName(name string) (Pipeline, error) {
	for pipeline := BuildDeploy; pipeline < numPipelines; pipeline++ {
		if pipeline.String() == name {
			return pipeline, nil
		}
	}

	return numPipelines, fmt.Errorf("No pipeline found by name %s", name)
}
