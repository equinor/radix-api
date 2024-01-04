package models

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"time"
)

// TLS
// swagger:model TLS
type TLS struct {
	// UseAutomation describes if TLS certificate is automatically issued using automation (ACME)
	//
	// required: true
	UseAutomation bool `json:"useAutomation"`

	// PrivateKey  status about the TLS private key
	//
	// required: true
	PrivateKey TLSPrivateKey `json:"privateKey"`

	// Certificate holds status about the TLS private key
	//
	// required: true
	Certificate TLSCertificate `json:"certificate"`
}

const (
	TLSPrivateKeyPending TLSPrivateKeyStatus = iota + 1
	TLSPrivateKeyConsistent
	TLSPrivateKeyInvalid
)

var (
	tlsPrivateKeyNames = map[TLSPrivateKeyStatus]string{
		TLSPrivateKeyPending:    "Pending",
		TLSPrivateKeyConsistent: "Consistent",
		TLSPrivateKeyInvalid:    "Invalid",
	}
)

// TLSPrivateKeyStatus Enum of TLS private key status
type TLSPrivateKeyStatus uint8

func (s *TLSPrivateKeyStatus) String() string {
	return tlsPrivateKeyNames[*s]
}

func (s *TLSPrivateKeyStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s *TLSPrivateKeyStatus) UnmarshalJSON(data []byte) error {
	var status string
	if err := json.Unmarshal(data, &status); err != nil {
		return err
	}

	for k, v := range tlsPrivateKeyNames {
		if v == status {
			*s = k
			return nil
		}
	}
	return fmt.Errorf("%q is not a valid status", status)
}

// TLSPrivateKey
// swagger:model TLSPrivateKey
type TLSPrivateKey struct {
	// Status of the private key
	// Pending: Private key is not set
	// Consistent: Private key is set and is valid
	// Invalid: Private key is set but is invalid
	//
	// required: true
	// enum: Pending,Consistent,Invalid
	// example: Consistent
	Status TLSPrivateKeyStatus `json:"status,omitempty"`

	// StatusMessages contains a list of messages related to the Status
	//
	// required: false
	StatusMessages []string `json:"statusMessages,omitempty"`
}

const (
	TLSCertificateStatusPending TLSCertificateStatus = iota + 1
	TLSCertificateStatusConsistent
	TLSCertificateStatusInvalid
)

var (
	tlsCertificateStatusNames = map[TLSCertificateStatus]string{
		TLSCertificateStatusPending:    "Pending",
		TLSCertificateStatusConsistent: "Consistent",
		TLSCertificateStatusInvalid:    "Invalid",
	}
)

// TLSCertificateStatus Enum of TLS certificate status
type TLSCertificateStatus uint8

func (s *TLSCertificateStatus) String() string {
	return tlsCertificateStatusNames[*s]
}

func (s *TLSCertificateStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s *TLSCertificateStatus) UnmarshalJSON(data []byte) error {
	var status string
	if err := json.Unmarshal(data, &status); err != nil {
		return err
	}

	for k, v := range tlsCertificateStatusNames {
		if v == status {
			*s = k
			return nil
		}
	}
	return fmt.Errorf("%q is not a valid status", status)
}

// TLSCertificate
// swagger:model TLSCertificate
type TLSCertificate struct {
	// Status of the certificate
	// Pending: Certificate is not set
	// Consistent: Certificate is set and is valid
	// Invalid: Certificate is set but is invalid
	//
	// required: true
	// enum: Pending,Consistent,Invalid
	// example: Consistent
	Status TLSCertificateStatus `json:"status,omitempty"`

	// StatusMessages contains a list of messages related to the Status
	//
	// required: false
	StatusMessages []string `json:"statusMessages,omitempty"`

	// X509Certificates holds the X509 certificate chain
	// The first certificate in the list should be the host certificate and the rest should be intermediate certificates
	//
	// required: false
	X509Certificates []X509Certificate `json:"x509Certificates,omitempty"`
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
