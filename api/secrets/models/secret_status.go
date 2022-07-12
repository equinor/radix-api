package models

// SecretStatus Enumeration of the statuses of a secret
type SecretStatus int

const (
	// Pending In configuration but not in cluster
	Pending SecretStatus = iota

	// Consistent In configuration and in cluster
	Consistent

	// Orphan In cluster and not in configuration
	Orphan

	// NotAvailable In external secret configuration but in cluster
	NotAvailable

	numStatuses
)

func (p SecretStatus) String() string {
	if p >= numStatuses {
		return "Unsupported"
	}
	return [...]string{"Pending", "Consistent", "Orphan", "NotAvailable"}[p]
}
