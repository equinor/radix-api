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
}

// UsedResource holds information about used resource
// swagger:model UsedResource
type UsedResource struct {
	// Min resource used
	//
	// required: false
	// example: 120m
	Min string `json:"min"`

	// Max resource used
	//
	// required: false
	// example: 120m
	Max string `json:"max"`

	// Average resource used
	//
	// required: false
	// example: 120m
	Average string `json:"average"`
}
