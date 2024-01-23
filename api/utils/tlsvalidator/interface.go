package tlsvalidator

// TLSSecretValidator defines methods to validate certificate and private key for TLS
type TLSSecretValidator interface {
	// ValidateTLSKey validates the private key
	// keyBytes must be in PEM format
	// Returns false is keyBytes is invalid, along with a list of validation error messages
	ValidateTLSKey(keyBytes []byte) (bool, []string)

	// ValidateTLSCertificate validates the certificate, dnsName and private key
	// certBytes and keyBytes must be in PEM format
	// Returns false if validation fails, along with a list of validation error messages
	ValidateTLSCertificate(certBytes, keyBytes []byte, dnsName string) (bool, []string)
}
