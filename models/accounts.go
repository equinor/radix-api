package models

import (
	"fmt"
	secretProviderClient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"

	radixmodels "github.com/equinor/radix-common/models"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	jwt "github.com/golang-jwt/jwt/v4"
	"k8s.io/client-go/kubernetes"
)

// NewAccounts creates a new Accounts struct
func NewAccounts(
	inClusterClient kubernetes.Interface,
	inClusterRadixClient radixclient.Interface,
	inClusterSecretProviderClient secretProviderClient.Interface,
	outClusterClient kubernetes.Interface,
	outClusterRadixClient radixclient.Interface,
	outClusterSecretProviderClient secretProviderClient.Interface,
	token string,
	impersonation radixmodels.Impersonation) Accounts {

	return Accounts{
		UserAccount: Account{
			Client:               outClusterClient,
			RadixClient:          outClusterRadixClient,
			SecretProviderClient: outClusterSecretProviderClient,
		},
		ServiceAccount: Account{
			Client:               inClusterClient,
			RadixClient:          inClusterRadixClient,
			SecretProviderClient: inClusterSecretProviderClient,
		},
		token:         token,
		impersonation: impersonation,
	}
}

func NewServiceAccount(inClusterClient kubernetes.Interface, inClusterRadixClient radixclient.Interface, inClusterSecretProviderClient secretProviderClient.Interface) Account {
	return Account{
		Client:               inClusterClient,
		RadixClient:          inClusterRadixClient,
		SecretProviderClient: inClusterSecretProviderClient,
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

func getUserPrincipleNameFromToken(token string) (string, error) {
	claims := jwt.MapClaims{}
	parser := jwt.Parser{}
	_, _, err := parser.ParseUnverified(token, claims)
	if err != nil {
		return "", fmt.Errorf("could not parse token (%v)", err)
	}

	userPrincipleName := fmt.Sprintf("%v", claims["upn"])
	return userPrincipleName, nil
}
