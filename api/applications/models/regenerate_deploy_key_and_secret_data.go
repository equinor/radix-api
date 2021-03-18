package models

// RegenerateDeployKeyAndSecretData Holds regenerated shared secret
// swagger:model RegenerateDeployKeyAndSecretData
type RegenerateDeployKeyAndSecretData struct {
	// SharedSecret of the shared secret
	//
	// required: true
	SharedSecret string `json:"sharedSecret"`
}
