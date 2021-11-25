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

	// ComponentOutdated has outdated image
	ComponentOutdated

	numComponentStatuses
)

func (p ComponentStatus) String() string {
	return [...]string{"Stopped", "Consistent", "Reconciling", "Restarting", "Outdated"}[p]
}

// GetComponentStatusFromName Gets status from name
func GetComponentStatusFromName(name string) (ComponentStatus, error) {
	for status := StoppedComponent; status < numComponentStatuses; status++ {
		if status.String() == name {
			return status, nil
		}
	}

	return numComponentStatuses, fmt.Errorf("no component status found by name %s", name)
}
