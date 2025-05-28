package models

// RegenerateDeployKeyData Holds regenerated shared secret
// swagger:model RegenerateDeployKeyData
type RegenerateDeployKeyData struct {
	// Deprecated: use RegenerateSharedSecretData instead
	// SharedSecret of the shared secret
	//
	// required: false
	SharedSecret string `json:"sharedSecret"`

	// PrivateKey of the deploy key
	//
	// required: false
	PrivateKey string `json:"privateKey"`
}
