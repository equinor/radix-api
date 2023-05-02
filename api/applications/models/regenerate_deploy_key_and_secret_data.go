package models

// RegenerateDeployKeyAndSecretData Holds regenerated shared secret
// swagger:model RegenerateDeployKeyAndSecretData
type RegenerateDeployKeyAndSecretData struct {
	// SharedSecret of the shared secret
	//
	// required: false
	SharedSecret string `json:"sharedSecret"`

	// PrivateKey of the deploy key
	//
	// required: false
	PrivateKey string `json:"privateKey"`
}
