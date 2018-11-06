package models

import "fmt"

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

func getStatus(name string) (ProgressStatus, error) {
	for status := Pending; status < numStatuses; status++ {
		if status.String() == name {
			return status, nil
		}
	}

	return numStatuses, fmt.Errorf("No progress status found by name %s", name)
}
