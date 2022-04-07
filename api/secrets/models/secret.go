package models

// Secret holds general information about secret
// swagger:model Secret
type Secret struct {
	// Name of the secret or its property, related to type and resource)
	//
	// required: true
	// example: db_password
	Name string `json:"name"`

	// DisplayName of the secret
	//
	// required: false
	// example: Database password
	DisplayName string `json:"displayName,omitempty"`

	// Type of the secret
	//
	// required: false
	// example: csi-az-blob
	Type SecretType `json:"type,omitempty"`

	// Resource of the secrets
	//
	// required: false
	// example: volumeAbc
	Resource string `json:"resource,omitempty"`

	// Component name of the component having the secret
	//
	// required: false
	// example: api
	Component string `json:"component,omitempty"`

	// Status of the secret
	// - Pending = Secret exists in Radix config, but not in cluster
	// - Consistent = Secret exists in Radix config and in cluster
	// - Orphan = Secret does not exist in Radix config, but exists in cluster
	//
	// required: false
	// example: Consistent
	Status string `json:"status,omitempty"`
}

type SecretType string

const (
	SecretTypeGeneric               SecretType = "generic"
	SecretTypeClientCert            SecretType = "client-cert"
	SecretTypeAzureBlobFuseVolume   SecretType = "azure-blob-fuse-volume"
	SecretTypeCsiAzureBlobVolume    SecretType = "csi-azure-blob-volume"
	SecretTypeCsiAzureKeyVaultCreds SecretType = "csi-azure-key-vault-creds"
	SecretTypeCsiAzureKeyVaultItem  SecretType = "csi-azure-key-vault-item"
	SecretTypeClientCertificateAuth SecretType = "client-cert-auth"
	SecretTypeOAuth2Proxy           SecretType = "oauth2-proxy"
	SecretTypeOrphaned              SecretType = "orphaned"
)

//GetSecretTypeDescription Gets description by the secret type
func GetSecretTypeDescription(secretType SecretType) string {
	switch secretType {
	case SecretTypeGeneric:
		return "Generic"
	case SecretTypeClientCert:
		return "TLS"
	case SecretTypeAzureBlobFuseVolume:
		return "Azure Blobfuse volume mount credential"
	case SecretTypeCsiAzureBlobVolume:
		return "Azure Blob volume mount credential"
	case SecretTypeCsiAzureKeyVaultCreds:
		return "Azure Key vault credential"
	case SecretTypeCsiAzureKeyVaultItem:
		return "Azure Key vault"
	case SecretTypeClientCertificateAuth:
		return "Authentication Client Certificate"
	case SecretTypeOAuth2Proxy:
		return "OAuth2 Proxy"
	case SecretTypeOrphaned:
		return "Orphaned"
	}
	return "Unsupported"
}
