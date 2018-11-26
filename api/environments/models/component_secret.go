package models

// ComponentSecret describes a component secret
// swagger:model ComponentSecret
type ComponentSecret struct {
	// Name the unique name of the Radix application deployment
	//
	// required: true
	// example: p4$sW0rDz
	SecretValue string `json:"secretValue"`
}
