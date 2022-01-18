package models

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
	return [...]string{"Pending", "Failing", "Running", "Terminated", "Starting"}[p]
}
