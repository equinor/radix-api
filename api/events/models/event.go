package models

import "github.com/go-openapi/strfmt"

// Event holds information about Kubernetes events
// swagger:model Event
type Event struct {

	// The time (ISO8601) at which the event was last recorded
	//
	// example: 2020-11-05T13:25:07.000Z
	LastTimestamp strfmt.DateTime `json:"lastTimestamp"`

	// Kind of object involved in this event
	//
	// example: Pod
	InvolvedObjectKind string `json:"involvedObjectKind"`

	// Namespavce of object involved in this event
	//
	// example: myapp-production
	InvolvedObjectNamespace string `json:"involvedObjectNamespace"`

	// Name of object involved in this event
	//
	// example: www-74cb7c986-fgcrl
	InvolvedObjectName string `json:"involvedObjectName"`

	// Type of this event (Normal, Warning)
	//
	// example: Warning
	Type string `json:"type"`

	// A should short, machine understandable string that gives the reason for this event
	//
	// example: Unhealthy
	Reason string `json:"reason"`

	// A human-readable description of the status of this event
	//
	// example: 'Readiness probe failed: dial tcp 10.40.1.5:3003: connect: connection refused'
	Message string `json:"message"`
}
