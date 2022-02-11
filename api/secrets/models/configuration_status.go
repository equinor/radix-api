package models

// ConfigurationStatus Enumeration of the statuses of configuration
type ConfigurationStatus int

const (
	// Pending In configuration but not in cluster
	Pending ConfigurationStatus = iota

	// Consistent In configuration and in cluster
	Consistent

	// Orphan In cluster and not in configuration
	Orphan

	// External Secret value is in external resource, status unknown
	External

	numStatuses
)

func (p ConfigurationStatus) String() string {
	if p >= numStatuses {
		return "Unsupported"
	}
	return [...]string{"Pending", "Consistent", "Orphan", "External"}[p]
}
