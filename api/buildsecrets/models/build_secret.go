package models

// BuildSecret holds general information about image hubs
// swagger:model BuildSecret
type BuildSecret struct {
	// Name name of the build secret
	//
	// required: true
	// example: SECRET_1
	Name string `json:"name"`

	// Status of the secret
	// - Pending = Secret value is not set
	// - Consistent = Secret value is set
	//
	// required: false
	// example: Consistent
	Status string `json:"status"`
}
