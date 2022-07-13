package models

// ADGroup describe a group
// swagger:model ADGroup
type ADGroup struct {
	// Group name
	//
	// required: true
	// example: fg_radix
	Name string `json:"name"`
	// Id of group
	//
	// required: true
	// example: abcdf-asdf-0000-a5a5-4abf6ae6f82e
	Id string `json:"id"`
}
