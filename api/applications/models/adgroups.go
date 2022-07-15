package models

// ADGroups list of ADGroups
// swagger:model ADGroups
type ADGroups struct {
	// List of ADGroups
	//
	// required: true
	// example:
	// id
	// name
	ADGroups []*ADGroup `json:"ADGroups"`
}
