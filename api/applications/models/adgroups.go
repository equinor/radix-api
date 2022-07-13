package models

// ADGroups list of ADGroups
// swagger:model ADGroups
type ADGroups struct {
	// List of ADGroups
	//
	// required: true
	// example: asd
	ADGroups []*ADGroup `json:"adgroups"`
}
