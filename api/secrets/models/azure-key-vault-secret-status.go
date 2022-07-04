package models

// AzureKeyVaultSecretStatus holds status of a Azure Key vault secret
// swagger:model AzureKeyVaultSecretStatus
type AzureKeyVaultSecretStatus struct {
	// Status of the secret
	//
	// required: true
	// example: Consistent
	Status string `json:"status"`

	// Versions of the secret
	//
	// required: false
	Versions []AzureKeyVaultSecretVersion `json:"versions,omitempty"`
}

// AzureKeyVaultSecretVersion holds a version of a Azure Key vault secret
// swagger:model AzureKeyVaultSecretVersion
type AzureKeyVaultSecretVersion struct {
	// ReplicaName which uses the secret
	//
	// required: true
	// example: abcdf
	ReplicaName string `json:"replicaName"`

	// Version of the secret
	//
	// required: true
	// example: 0123456789
	Version string `json:"version"`
}
