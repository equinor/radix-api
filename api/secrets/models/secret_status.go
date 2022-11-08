package models

// SecretStatus Enumeration of the statuses of a secret
type SecretStatus int

const (
	// Pending In configuration but not in cluster
	Pending SecretStatus = iota

	// Consistent In configuration and in cluster
	Consistent

	// NotAvailable In external secret configuration but in cluster
	NotAvailable

	// Invalid when secret value is set, but the format of the value is invalid
	Invalid

	numStatuses
)

func (p SecretStatus) String() string {
	if p >= numStatuses {
		return "Unsupported"
	}
	return [...]string{"Pending", "Consistent", "NotAvailable", "Invalid"}[p]
}
