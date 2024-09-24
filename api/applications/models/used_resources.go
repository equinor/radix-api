package models

// UsedResources holds information about used resources
// swagger:model UsedResources
type UsedResources struct {
	// From timestamp
	//
	// required: true
	// example: 2006-01-02T15:04:05Z
	From string `json:"from"`

	// To timestamp
	//
	// required: true
	// example: 2006-01-02T15:04:05Z
	To string `json:"to"`

	// CPU used, in cores
	//
	// required: false
	CPU *UsedResource `json:"cpu,omitempty"`

	// Memory used, in bytes
	//
	// required: false
	Memory *UsedResource `json:"memory,omitempty"`

	// Warning messages
	//
	// required: false
	Warnings []string `json:"warnings,omitempty"`
}

// UsedResource holds information about used resource
// swagger:model UsedResource
type UsedResource struct {
	// Min actual precise resource used
	//
	// required: false
	// example: 0.00012
	Min *float64 `json:"min,omitempty"`

	// Max actual precise resource used
	//
	// required: false
	// example: 0.00037
	Max *float64 `json:"max,omitempty"`

	// Avg actual precise resource used
	//
	// required: false
	// example: 0.00012
	Avg *float64 `json:"avg,omitempty"`
}
