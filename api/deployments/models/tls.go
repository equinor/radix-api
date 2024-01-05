package models

import (
	"crypto/x509"
	"encoding/pem"
	"time"
)

// swagger:enum TLSStatus
type TLSStatus string

const (
	TLSStatusPending    TLSStatus = "Pending"
	TLSStatusConsistent TLSStatus = "Consistent"
	TLSStatusInvalid    TLSStatus = "Invalid"
)

// TLS configuration and status for external DNS
// swagger:model TLS
type TLS struct {
	// UseAutomation describes if TLS certificate is automatically issued using automation (ACME)
	//
	// required: true
	UseAutomation bool `json:"useAutomation"`

	// Status of the private key
	// Pending: Private key is not set
	// Consistent: Private key is set and is valid
	// Invalid: Private key is set but is invalid
	//
	// required: true
	// enum: Pending,Consistent,Invalid
	// example: Consistent
	PrivateKeyStatus TLSStatus `json:"privateKeyStatus"`

	// PrivateKeyStatusMessages contains a list of messages related to PrivateKeyStatus
	//
	// required: false
	PrivateKeyStatusMessages []string `json:"privateKeyStatusMessages,omitempty"`

	// Status of the certificate
	// Pending: Certificate is not set
	// Consistent: Certificate is set and is valid
	// Invalid: Certificate is set but is invalid
	//
	// required: true
	// enum: Pending,Consistent,Invalid
	// example: Consistent
	CertificateStatus TLSStatus `json:"certificateStatus"`

	// CertificateStatusMessages contains a list of messages related to CertificateStatus
	//
	// required: false
	CertificateStatusMessages []string `json:"certificateStatusMessages,omitempty"`

	// Certificates holds the X509 certificate chain
	// The first certificate in the list should be the host certificate and the rest should be intermediate certificates
	//
	// required: false
	Certificates []X509Certificate `json:"certificates,omitempty"`
}

// X509Certificate holds information about a X509 certificate
// swagger:model X509Certificate
type X509Certificate struct {
	// Subject contains the distinguished name for the certificate
	//
	// required: true
	// example: CN=mysite.example.com,O=MyOrg,L=MyLocation,C=NO
	Subject string `json:"subject"`
	// Issuer contains the distinguished name for the certificate's issuer
	//
	// required: true
	// example: CN=DigiCert TLS RSA SHA256 2020 CA1,O=DigiCert Inc,C=US
	Issuer string `json:"issuer"`
	// NotBefore defines the lower date/time validity boundary
	//
	// required: true
	// swagger:strfmt date-time
	// example: 2022-08-09T00:00:00Z
	NotBefore time.Time `json:"notBefore"`
	// NotAfter defines the uppdater date/time validity boundary
	//
	// required: true
	// swagger:strfmt date-time
	// example: 2023-08-25T23:59:59Z
	NotAfter time.Time `json:"notAfter"`
	// DNSNames defines list of Subject Alternate Names in the certificate
	//
	// required: false
	DNSNames []string `json:"dnsNames,omitempty"`
}

// ParseX509CertificatesFromPEM builds an array of X509Certificate from PEM encoded data
func ParseX509CertificatesFromPEM(certBytes []byte) []X509Certificate {
	var certs []X509Certificate
	for len(certBytes) > 0 {
		var block *pem.Block
		block, certBytes = pem.Decode(certBytes)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}

		certs = append(certs, X509Certificate{
			Subject:   cert.Subject.String(),
			Issuer:    cert.Issuer.String(),
			DNSNames:  cert.DNSNames,
			NotBefore: cert.NotBefore,
			NotAfter:  cert.NotAfter,
		})
	}

	return certs
}
