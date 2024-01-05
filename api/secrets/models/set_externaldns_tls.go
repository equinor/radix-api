package models

// SetExternalDNSTLSRequest describes request body for setting private key and certificate for external DNS TLS
// swagger:model SetExternalDNSTLSRequest
type SetExternalDNSTLSRequest struct {
	// Private key in PEM format
	//
	// required: true
	PrivateKey string `json:"privateKey"`

	// X509 certificate in PEM format
	//
	// required: true
	Certificate string `json:"certificate"`
}
