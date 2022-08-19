package models

// AzureKeyVaultSecretVersion holds a version of a Azure Key vault secret
// swagger:model AzureKeyVaultSecretVersion
type AzureKeyVaultSecretVersion struct {
	// ReplicaName which uses the secret
	//
	// required: true
	// example: abcdf
	ReplicaName string `json:"replicaName"`

	// JobName which uses the secret
	//
	// required: true
	// example: job-abc
	JobName string `json:"jobName"`

	// BatchName which uses the secret
	//
	// required: true
	// example: batch-abc
	BatchName string `json:"batchName"`

	// Version of the secret
	//
	// required: true
	// example: 0123456789
	Version string `json:"version"`
}
