package models

import (
	"fmt"

	kedav2 "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"

	certclient "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	radixmodels "github.com/equinor/radix-common/models"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/golang-jwt/jwt/v5"
	"k8s.io/client-go/kubernetes"
	secretProviderClient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
)

// NewAccounts creates a new Accounts struct
func NewAccounts(
	inClusterClient kubernetes.Interface, inClusterRadixClient radixclient.Interface, inClusterKedaClient kedav2.Interface, inClusterSecretProviderClient secretProviderClient.Interface, inClusterTektonClient tektonclient.Interface, inClusterCertManagerClient certclient.Interface,
	outClusterClient kubernetes.Interface, outClusterRadixClient radixclient.Interface, outClusterKedaClient kedav2.Interface, outClusterSecretProviderClient secretProviderClient.Interface, outClusterTektonClient tektonclient.Interface, outClusterCertManagerClient certclient.Interface,
	token string, impersonation radixmodels.Impersonation) Accounts {

	return Accounts{
		UserAccount: Account{
			Client:               outClusterClient,
			RadixClient:          outClusterRadixClient,
			KedaClient:           outClusterKedaClient,
			SecretProviderClient: outClusterSecretProviderClient,
			TektonClient:         outClusterTektonClient,
			CertManagerClient:    outClusterCertManagerClient,
		},
		ServiceAccount: Account{
			Client:               inClusterClient,
			RadixClient:          inClusterRadixClient,
			KedaClient:           inClusterKedaClient,
			SecretProviderClient: inClusterSecretProviderClient,
			TektonClient:         inClusterTektonClient,
			CertManagerClient:    inClusterCertManagerClient,
		},
		token:         token,
		impersonation: impersonation,
	}
}

func NewServiceAccount(inClusterClient kubernetes.Interface, inClusterRadixClient radixclient.Interface, inClusterKedaClient kedav2.Interface, inClusterSecretProviderClient secretProviderClient.Interface, inClusterTektonClient tektonclient.Interface, inClusterCertManagerClient certclient.Interface) Account {
	return Account{
		Client:               inClusterClient,
		RadixClient:          inClusterRadixClient,
		SecretProviderClient: inClusterSecretProviderClient,
		TektonClient:         inClusterTektonClient,
		CertManagerClient:    inClusterCertManagerClient,
		KedaClient:           inClusterKedaClient,
	}
}

// Accounts contains accounts for accessing k8s API.
type Accounts struct {
	UserAccount    Account
	ServiceAccount Account
	token          string
	impersonation  radixmodels.Impersonation
}

// GetOriginator get the request originator name or id
func (accounts Accounts) GetOriginator() (string, error) {
	if accounts.impersonation.PerformImpersonation() {
		return accounts.impersonation.User, nil
	}
	if originator, err, done := accounts.getOriginator("upn", ""); done {
		return originator, err
	}
	if originator, err, done := accounts.getOriginator("app_displayname", ""); done {
		return originator, err
	}
	if originator, err, done := accounts.getOriginator("appid", "%s (appid)"); done {
		return originator, err
	}
	if originator, err, done := accounts.getOriginator("sub", "%s (sub)"); done {
		return originator, err
	}
	return "", nil
}

func (accounts Accounts) getOriginator(claim, format string) (string, error, bool) {
	originator, err := getTokenClaim(accounts.token, claim)
	if err != nil {
		return "", err, true
	}
	if originator == "" {
		return "", nil, false
	}
	if format != "" {
		return fmt.Sprintf(format, originator), nil, true
	}
	return originator, nil, true
}

func getTokenClaim(token string, claim string) (string, error) {
	claims := jwt.MapClaims{}
	parser := jwt.Parser{}
	_, _, err := parser.ParseUnverified(token, claims)
	if err != nil {
		return "", fmt.Errorf("could not parse token (%v)", err)
	}
	if val, ok := claims[claim]; ok {
		return fmt.Sprintf("%v", val), nil
	}
	return "", nil
}
