package models

import (
	"fmt"

	radixhttp "github.com/equinor/radix-common/net/http"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// PipelineNotFoundError Pipeline job not found
func PipelineNotFoundError(appName, jobName string) error {
	return radixhttp.TypeMissingError(fmt.Sprintf("job %s not found for the app %s", jobName, appName), nil)
}

// PipelineStepNotFoundError Pipeline job step not found
func PipelineStepNotFoundError(appName, jobName, stepName string) error {
	return radixhttp.TypeMissingError(fmt.Sprintf("step %s for the job %s not found for the app %s", stepName, jobName, appName), nil)
}

// JobHasInvalidConditionToRerunError Pipeline job cannot be rerun due to invalid condition
func JobHasInvalidConditionToRerunError(appName, jobName string, jobCondition v1.RadixJobCondition) error {
	return radixhttp.ValidationError("Radix Application Pipeline", fmt.Sprintf("only Failed or Stopped pipeline jobs can be rerun, the job %s for the app %s has status %s", appName, jobName, jobCondition))
}

// JobAlreadyRequestedToStopError Pipeline job was already requested to stop
func JobAlreadyRequestedToStopError(appName, jobName string) error {
	return radixhttp.ValidationError("Radix Application Pipeline", fmt.Sprintf("job %s for the app %s is already requested to stop", appName, jobName))
}

// JobHasInvalidConditionToStopError Pipeline job cannot be stopped due to invalid condition
func JobHasInvalidConditionToStopError(appName, jobName string, jobCondition v1.RadixJobCondition) error {
	return radixhttp.ValidationError("Radix Application Pipeline", fmt.Sprintf("only not Failed or Stopped pipeline jobs can be stopped, the job %s for the app %s has status %s", appName, jobName, jobCondition))
}
