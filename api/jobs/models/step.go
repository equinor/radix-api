package models

import v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"

// VulnerabilityScan holds information about vulnerabilities found during scan
// swagger:model VulnerabilityScan
type VulnerabilityScan struct {
	// Status of the vulnerability scan
	//
	// required: true
	// Enum: Success,Missing
	// example: Success
	Status v1.ScanStatus `json:"status,omitempty"`

	// Reason for the status
	//
	// required: false
	// example: Scan results not found in output from scan job
	Reason string `json:"reason,omitempty"`

	// Overview of severities and count from list of vulnerabilities
	//
	// required: false
	Vulnerabilities v1.VulnerabilityMap `json:"vulnerabilities,omitempty"`
}

// Step holds general information about job step
// swagger:model Step
type Step struct {
	// Name of the step
	//
	// required: false
	// example: build
	Name string `json:"name"`

	// Status of the step
	//
	// required: false
	// Enum: Waiting,Running,Succeeded,Failed
	// example: Waiting
	Status string `json:"status"`

	// Started timestamp
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	Started string `json:"started"`

	// Ended timestamp
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	Ended string `json:"ended"`

	// Pod name
	//
	// required: false
	PodName string `json:"-"`

	// Components associated components
	//
	// required: false
	Components []string `json:"components,omitempty"`

	// Information about vulnerabilities found in scan step
	//
	// required: false
	VulnerabilityScan *VulnerabilityScan `json:"scan,omitempty"`
}
