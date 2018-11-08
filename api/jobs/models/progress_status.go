package models

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
)

// ProgressStatus Enumeration of the statuses of a job or step
type ProgressStatus int

const (
	// Pending Awaiting start
	Pending ProgressStatus = iota

	// Running Started
	Running

	// Success Job/step succeeded
	Success

	// Fail Job/step failed
	Fail

	numStatuses
)

func (p ProgressStatus) String() string {
	return [...]string{"Pending", "Running", "Success", "Fail"}[p]
}

// GetStatusFromName Gets status from name
func GetStatusFromName(name string) (ProgressStatus, error) {
	for status := Pending; status < numStatuses; status++ {
		if status.String() == name {
			return status, nil
		}
	}

	return numStatuses, fmt.Errorf("No progress status found by name %s", name)
}

// GetStatusFromJobStatus Gets status from kubernetes job status
func GetStatusFromJobStatus(jobStatus batchv1.JobStatus) ProgressStatus {
	status := Pending
	if jobStatus.Active > 0 {
		status = Running

	} else if jobStatus.Succeeded > 0 {
		status = Success

	} else if jobStatus.Failed > 0 {
		status = Fail
	}

	return status
}
