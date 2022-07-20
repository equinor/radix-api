package models

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
