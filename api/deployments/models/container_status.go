package models

import (
	"fmt"
)

// ContainerStatus Enumeration of the statuses of container
type ContainerStatus int

const (
	// Pending container
	Pending ContainerStatus = iota

	// Failing container
	Failing

	// Running container
	Running

	// Terminated container
	Terminated

	// Starting container
	Starting

	numStatuses
)

func (p ContainerStatus) String() string {
	return [...]string{"Pending", "Failing", "Running", "Terminated"}[p]
}

// GetStatusFromName Gets status from name
func GetStatusFromName(name string) (ContainerStatus, error) {
	for status := Pending; status < numStatuses; status++ {
		if status.String() == name {
			return status, nil
		}
	}

	return numStatuses, fmt.Errorf("No container status found by name %s", name)
}
