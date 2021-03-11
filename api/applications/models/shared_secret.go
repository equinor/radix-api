package models

// SharedSecret Holds regenerated shared secret
// swagger:model SharedSecret
type SharedSecret struct {
	// Value of the shared secret
	//
	// required: true
	Value string `json:"value"`
}
