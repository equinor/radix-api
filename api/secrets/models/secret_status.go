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

	// Invalid value. The vaulue is set, but the format is incorrect
	Invalid

	numStatuses
)

func (p SecretStatus) String() string {
	if p >= numStatuses {
		return "Unsupported"
	}
	return [...]string{"Pending", "Consistent", "NotAvailable", "Invalid"}[p]
}
