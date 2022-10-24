package secrets

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
)

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

type tlsSecretValidator struct{}

func (v *tlsSecretValidator) ValidateTLSKey(keyBytes []byte) (valid bool, failedValidationMessages []string) {
	defer func() {
		valid = len(failedValidationMessages) == 0
	}()
	keyBlock, _ := pem.Decode(keyBytes)
	if keyBlock == nil {
		failedValidationMessages = append(failedValidationMessages, "tls: failed to find any PEM data in key input")
		return
	}

	_, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		failedValidationMessages = append(failedValidationMessages, err.Error())
	}

	return
}

func (v *tlsSecretValidator) ValidateTLSCertificate(certBytes, keyBytes []byte, dnsName string) (valid bool, failedValidationMessages []string) {
	defer func() {
		valid = len(failedValidationMessages) == 0
	}()

	certblock, intermediatBytes := pem.Decode(certBytes)
	if certblock == nil || certblock.Type != "CERTIFICATE" {
		failedValidationMessages = append(failedValidationMessages, "x509: missing PEM block for certificate")
		return
	}

	cert, err := x509.ParseCertificate(certblock.Bytes)
	if err != nil {
		failedValidationMessages = append(failedValidationMessages, err.Error())
		return
	}

	_, err = tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		failedValidationMessages = append(failedValidationMessages, err.Error())
	}

	intermediatePool := x509.NewCertPool()
	intermediatePool.AppendCertsFromPEM(intermediatBytes)
	_, err = cert.Verify(x509.VerifyOptions{DNSName: dnsName, Intermediates: intermediatePool})
	if err != nil {
		failedValidationMessages = append(failedValidationMessages, err.Error())
	}

	return
}
