package models

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
	if p >= numStatuses {
		return "Unsupported"
	}
	return [...]string{"Pending", "Consistent"}[p]
}
