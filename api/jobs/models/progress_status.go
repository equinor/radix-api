package models

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
)

// ProgressStatus Enumeration of the statuses of a job or step
type ProgressStatus int

const (
	// Active Started
	Active ProgressStatus = iota

	// Succeeded Job/step succeeded
	Succeeded

	// Failed Job/step failed
	Failed

	// Waiting Job/step pending
	Waiting

	numStatuses
)

func (p ProgressStatus) String() string {
	return [...]string{"Active", "Succeeded", "Failed"}[p]
}

// GetStatusFromName Gets status from name
func GetStatusFromName(name string) (ProgressStatus, error) {
	for status := Active; status < numStatuses; status++ {
		if status.String() == name {
			return status, nil
		}
	}

	return numStatuses, fmt.Errorf("No progress status found by name %s", name)
}

// GetStatusFromJobStatus Gets status from kubernetes job status
func GetStatusFromJobStatus(jobStatus batchv1.JobStatus) ProgressStatus {
	var status ProgressStatus
	if jobStatus.Active > 0 {
		status = Active

	} else if jobStatus.Succeeded > 0 {
		status = Succeeded

	} else if jobStatus.Failed > 0 {
		status = Failed
	}

	return status
}
