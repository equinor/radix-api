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
	// required: false
	// example: CN=mysite.example.com,O=MyOrg,L=MyLocation,C=NO
	Subject string `json:"subject,omitempty"`
	// Issuer contains the distinguished name for the certificate's issuer
	//
	// required: false
	// example: CN=DigiCert TLS RSA SHA256 2020 CA1,O=DigiCert Inc,C=US
	Issuer string `json:"issuer,omitempty"`
	// NotBefore defines the lower date/time validity boundary
	//
	// required: false
	// example: 2022-08-09T00:00:00Z
	NotBefore *time.Time `json:"notBefore,omitempty"`
	// NotAfter defines the uppdater date/time validity boundary
	//
	// required: false
	// example: 2023-08-25T23:59:59Z
	NotAfter *time.Time `json:"notAfter,omitempty"`
	// DNSNames defines list of Subject Alternate Names in the certificate
	//
	// required: false
	DNSNames []string `json:"dnsNames,omitempty"`
}

func ParseTLSCertificate(certBytes []byte) (*TLSCertificate, error) {
	certblock, _ := pem.Decode(certBytes)
	if certblock == nil || certblock.Type != "CERTIFICATE" {
		return nil, errors.New("x509: missing PEM block for certificate")
	}

	cert, err := x509.ParseCertificate(certblock.Bytes)
	if err != nil {
		return nil, err
	}

	certInfo := TLSCertificate{
		Subject:   cert.Subject.String(),
		Issuer:    cert.Issuer.String(),
		DNSNames:  cert.DNSNames,
		NotBefore: &cert.NotBefore,
		NotAfter:  &cert.NotAfter,
	}

	return &certInfo, nil
}
