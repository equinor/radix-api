package models

import (
	"fmt"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"

	radixmodels "github.com/equinor/radix-common/models"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/golang-jwt/jwt/v4"
	"k8s.io/client-go/kubernetes"
	secretProviderClient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
)

// NewAccounts creates a new Accounts struct
func NewAccounts(inClusterClient kubernetes.Interface, inClusterRadixClient radixclient.Interface, inClusterSecretProviderClient secretProviderClient.Interface, inClusterTektonClient tektonclient.Interface, outClusterClient kubernetes.Interface, outClusterRadixClient radixclient.Interface, outClusterSecretProviderClient secretProviderClient.Interface, outClusterTektonClient tektonclient.Interface, token string, impersonation radixmodels.Impersonation) Accounts {

	return Accounts{
		UserAccount: Account{
			Client:               outClusterClient,
			RadixClient:          outClusterRadixClient,
			SecretProviderClient: outClusterSecretProviderClient,
			TektonClient:         outClusterTektonClient,
		},
		ServiceAccount: Account{
			Client:               inClusterClient,
			RadixClient:          inClusterRadixClient,
			SecretProviderClient: inClusterSecretProviderClient,
			TektonClient:         inClusterTektonClient,
		},
		token:         token,
		impersonation: impersonation,
	}
}

func NewServiceAccount(inClusterClient kubernetes.Interface, inClusterRadixClient radixclient.Interface, inClusterSecretProviderClient secretProviderClient.Interface, inClusterTektonClient tektonclient.Interface) Account {
	return Account{
		Client:               inClusterClient,
		RadixClient:          inClusterRadixClient,
		SecretProviderClient: inClusterSecretProviderClient,
		TektonClient:         inClusterTektonClient,
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
func (accounts Accounts) GetOriginator(isDebugMode bool) (string, error) {
	if isDebugMode {
		return "debug_mode", nil
	}
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
