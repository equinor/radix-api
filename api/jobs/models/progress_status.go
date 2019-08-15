package models

import (
	"fmt"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	batchv1 "k8s.io/api/batch/v1"
)

// UnscheduledJobStatus radix-operator still hasn't picked up the job
const UnscheduledJobStatus = "Unscheduled"

// ProgressStatus Enumeration of the statuses of a job or step
type ProgressStatus int

const (
	// Running Active
	Running ProgressStatus = iota

	// Succeeded Job/step succeeded
	Succeeded

	// Failed Job/step failed
	Failed

	// Waiting Job/step pending
	Waiting

	numStatuses
)

func (p ProgressStatus) String() string {
	return [...]string{"Running", "Succeeded", "Failed", "Waiting"}[p]
}

// GetStatusFromName Gets status from name
func GetStatusFromName(name string) (ProgressStatus, error) {
	for status := Running; status < numStatuses; status++ {
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
		status = Running

	} else if jobStatus.Succeeded > 0 {
		status = Succeeded

	} else if jobStatus.Failed > 0 {
		status = Failed
	}

	return status
}

// GetStatusFromRadixJobStatus Returns job status as string
func GetStatusFromRadixJobStatus(jobStatus v1.RadixJobStatus) string {
	if jobStatus.Condition != "" {
		return string(jobStatus.Condition)
	}

	return UnscheduledJobStatus
}
