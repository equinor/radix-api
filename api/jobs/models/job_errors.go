package models

import (
	"fmt"

	"github.com/equinor/radix-api/api/utils"
)

// PipelineNotFoundError Job not found
func PipelineNotFoundError(appName, jobName string) error {
	return utils.TypeMissingError(fmt.Sprintf("Job %s not found for app %s", jobName, appName), nil)
}
