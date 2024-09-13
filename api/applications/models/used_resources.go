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

	// CPU used
	//
	// required: false
	// example: 120m
	CPU *UsedResource `json:"cpu,omitempty"`

	// CPU used
	//
	// required: false
	// example: 120m
	Memory *UsedResource `json:"memory,omitempty"`

	// Warning messages
	//
	// required: false
	Warnings []string `json:"warnings,omitempty"`
}

// UsedResource holds information about used resource
// swagger:model UsedResource
type UsedResource struct {
	// Min resource used
	//
	// required: false
	// example: 120m
	Min string `json:"min,omitempty"`

	// Max resource used
	//
	// required: false
	// example: 120m
	Max string `json:"max,omitempty"`

	// Average resource used
	//
	// required: false
	// example: 120m
	Average string `json:"average,omitempty"`

	// MinActual actual precise resource used
	//
	// required: false
	// example: 0.00012
	MinActual *float64 `json:"minActual,omitempty"`

	// MaxActual actual precise resource used
	//
	// required: false
	// example: 0.00037
	MaxActual *float64 `json:"maxActual,omitempty"`

	// AvgActual actual precise resource used
	//
	// required: false
	// example: 0.00012
	AvgActual *float64 `json:"avgActual,omitempty"`
}
