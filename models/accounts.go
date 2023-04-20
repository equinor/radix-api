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

// GetUserAccountUserPrincipleName get the user principle name represented in UserAccount
func (accounts Accounts) GetUserAccountUserPrincipleName() (string, error) {
	if accounts.impersonation.PerformImpersonation() {
		return accounts.impersonation.User, nil
	}

	return getUserPrincipleNameFromToken(accounts.token)
}

// GetServicePrincipalAppIdFromToken get the service principal app id represented in a token
func (accounts Accounts) GetServicePrincipalAppIdFromToken() (string, error) {
	return getTokenClaim(accounts.token, "appId")
}

func getUserPrincipleNameFromToken(token string) (string, error) {
	return getTokenClaim(token, "upn")
}

func getTokenClaim(token string, claim string) (string, error) {
	claims := jwt.MapClaims{}
	parser := jwt.Parser{}
	_, _, err := parser.ParseUnverified(token, claims)
	if err != nil {
		return "", fmt.Errorf("could not parse token (%v)", err)
	}

	return fmt.Sprintf("%v", claims[claim]), nil
}
