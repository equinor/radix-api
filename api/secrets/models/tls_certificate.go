package models

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"time"
)

// TLSCertificate holds information about a TLS certificate
// swagger:model TLSCertificate
type TLSCertificate struct {
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
	// example: 2022-08-09T00:00:00Z
	NotBefore time.Time `json:"notBefore"`
	// NotAfter defines the uppdater date/time validity boundary
	//
	// required: true
	// example: 2023-08-25T23:59:59Z
	NotAfter time.Time `json:"notAfter"`
	// DNSNames defines list of Subject Alternate Names in the certificate
	//
	// required: false
	DNSNames []string `json:"dnsNames,omitempty"`
}

// ParseTLSCertificatesFromPEM builds an array TLSCertificate from PEM encoded data
func ParseTLSCertificatesFromPEM(certBytes []byte) ([]TLSCertificate, error) {
	var certs []TLSCertificate
	for len(certBytes) > 0 {
		certblock, rest := pem.Decode(certBytes)
		certBytes = rest
		if certblock == nil || certblock.Type != "CERTIFICATE" {
			return nil, errors.New("x509: missing PEM block for certificate")
		}

		cert, err := x509.ParseCertificate(certblock.Bytes)
		if err != nil {
			return nil, err
		}

		certs = append(certs, TLSCertificate{
			Subject:   cert.Subject.String(),
			Issuer:    cert.Issuer.String(),
			DNSNames:  cert.DNSNames,
			NotBefore: cert.NotBefore,
			NotAfter:  cert.NotAfter,
		})
	}

	return certs, nil
}
