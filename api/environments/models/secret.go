package models

// Secret holds general information about secret
// swagger:model Secret
type Secret struct {
	// Name of the secret
	//
	// required: false
	// example: db_password
	Name string `json:"name"`

	// Component name of the component having the secret
	//
	// required: false
	// example: api
	Component string `json:"component"`

	// Status of the secret
	// - Pending = Secret exists in Radix config, but not in cluster
	// - Consistent = Secret exists in Radix config and in cluster
	// - Orphan = Secret does not exist in Radix config, but exists in cluster
	//
	// required: false
	// example: Consistent
	Status string `json:"status"`
}
