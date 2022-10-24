package models

import "fmt"

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

	// ID of the secret within the Resource
	//
	// required: false
	// example: clientId
	ID string `json:"id,omitempty"`

	// Component name of the component having the secret
	//
	// required: false
	// example: api
	Component string `json:"component,omitempty"`

	// Status of the secret
	// - Pending = Secret exists in Radix config, but not in cluster
	// - Consistent = Secret exists in Radix config and in cluster
	// - NotAvailable = Secret is available in external secret configuration but not in cluster
	//
	// required: false
	// example: Consistent
	Status string `json:"status,omitempty"`

	// StatusMessages contains a list of messages related to the Status
	//
	// required: false
	StatusMessages []string `json:"statusMessages,omitempty"`

	// TLSCertificates holds the TLS certificate and certificate authorities (CA)
	// The first certificate in the list should the TLS certificate and the rest should be CAs
	//
	// required: false
	TLSCertificates []TLSCertificate `json:"tlsCertificates,omitempty"`
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
)

const (
	SecretIdKey          string = "key"
	SecretIdCert         string = "cert"
	SecretIdClientId     string = "clientId"
	SecretIdClientSecret string = "clientSecret"
	SecretIdAccountName  string = "accountName"
	SecretIdAccountKey   string = "accountKey"
)

func (secret Secret) String() string {
	return fmt.Sprintf("ID: %s, resource: %s, name: %s", secret.ID, secret.Resource, secret.Name)
}
