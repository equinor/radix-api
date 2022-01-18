package models

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
