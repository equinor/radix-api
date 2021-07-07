package models

import (
	"fmt"

	radixhttp "github.com/equinor/radix-common/net/http"
)

// PipelineNotFoundError Job not found
func PipelineNotFoundError(appName, jobName string) error {
	return radixhttp.TypeMissingError(fmt.Sprintf("Job %s not found for app %s", jobName, appName), nil)
}
