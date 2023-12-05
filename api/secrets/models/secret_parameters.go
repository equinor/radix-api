package models

// SecretParameters describes a component secret
// swagger:model SecretParameters
type SecretParameters struct {
	// Name the unique name of the Radix application deployment
	//
	// required: true
	// example: p4$sW0rDz
	SecretValue string `json:"secretValue"`

	// Type of the secret
	//
	// required: false
	// enum: generic,client-cert,azure-blob-fuse-volume,csi-azure-blob-volume,csi-azure-key-vault-creds,csi-azure-key-vault-item,client-cert-auth,oauth2-proxy
	// example: azure-blob-fuse-volume
	Type SecretType `json:"type,omitempty"`
}
