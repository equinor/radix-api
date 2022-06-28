package models

// AzureKeyVaultSecretStatus holds status of a Azure Key vault secret
// swagger:model AzureKeyVaultSecretStatus
type AzureKeyVaultSecretStatus struct {
	// Name of the secret or its property, related to type and resource)
	//
	// required: true
	// example: secret/some-name
	Name string `json:"name"`

	// PodName used the secret
	//
	// required: true
	// example: abcdf
	PodName string `json:"podName"`

	// Version of the secret
	//
	// required: true
	// example: 0123456789
	Version string `json:"version"`
}
