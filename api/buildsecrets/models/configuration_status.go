package models

import (
	"fmt"
)

// ConfigurationStatus Enumeration of the statuses of configuration
type ConfigurationStatus int

const (
	// Pending In configuration but not in cluster
	Pending ConfigurationStatus = iota

	// Consistent In configuration and in cluster
	Consistent

	numStatuses
)

func (p ConfigurationStatus) String() string {
	return [...]string{"Pending", "Consistent"}[p]
}

// GetStatusFromName Gets status from name
func GetStatusFromName(name string) (ConfigurationStatus, error) {
	for status := Pending; status < numStatuses; status++ {
		if status.String() == name {
			return status, nil
		}
	}

	return numStatuses, fmt.Errorf("No configuration status found by name %s", name)
}
