package models

import "fmt"

// ComponentStatus Enumeration of the statuses of component
type ComponentStatus int

const (
	// StoppedComponent stopped component
	StoppedComponent ComponentStatus = iota

	// ConsistentComponent consistent component
	ConsistentComponent

	// ComponentReconciling Component reconciling
	ComponentReconciling

	// ComponentRestarting restarting component
	ComponentRestarting

	numComponentStatuses
)

func (p ComponentStatus) String() string {
	return [...]string{"Stopped", "Consistent", "Reconciling", "Restarting"}[p]
}

// GetComponentStatusFromName Gets status from name
func GetComponentStatusFromName(name string) (ComponentStatus, error) {
	for status := StoppedComponent; status < numComponentStatuses; status++ {
		if status.String() == name {
			return status, nil
		}
	}

	return numComponentStatuses, fmt.Errorf("No component status found by name %s", name)
}
