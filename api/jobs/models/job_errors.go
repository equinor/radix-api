package models

import (
	"fmt"

	radixhttp "github.com/equinor/radix-common/net/http"
)

// PipelineNotFoundError Pipeline job not found
func PipelineNotFoundError(appName, jobName string) error {
	return radixhttp.TypeMissingError(fmt.Sprintf("job %s not found for the app %s", jobName, appName), nil)
}

// PipelineStepNotFoundError Pipeline job step not found
func PipelineStepNotFoundError(appName, jobName, stepName string) error {
	return radixhttp.TypeMissingError(fmt.Sprintf("step %s for the job %s not found for the app %s", stepName, jobName, appName), nil)
}

// PipelineRunNotFoundError Tekton PipelineRun not found for the pipeline job
func PipelineRunNotFoundError(appName, jobName, pipelineRunName string) error {
	return radixhttp.TypeMissingError(fmt.Sprintf("pipeline run %s not found for the app %s and the pipeline job %s", pipelineRunName, appName, jobName), nil)
}
