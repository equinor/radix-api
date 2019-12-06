package models

import (
	"fmt"
	"net/http"

	jwt "github.com/dgrijalva/jwt-go"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
)

// NewAccounts creates a new Accounts struct
func NewAccounts(
	inClusterClient kubernetes.Interface,
	inClusterRadixClient radixclient.Interface,
	outClusterClient kubernetes.Interface,
	outClusterRadixClient radixclient.Interface,
	token string,
	impersonation Impersonation) Accounts {

	return Accounts{
		UserAccount: Account{
			Client:      outClusterClient,
			RadixClient: outClusterRadixClient,
		},
		ServiceAccount: Account{
			Client:      inClusterClient,
			RadixClient: inClusterRadixClient,
		},
		token:         token,
		impersonation: impersonation,
	}
}

// Accounts contains accounts for accessing k8s API.
type Accounts struct {
	UserAccount    Account
	ServiceAccount Account
	token          string
	impersonation  Impersonation
}

// RadixHandlerFunc Pattern for handler functions
type RadixHandlerFunc func(Accounts, http.ResponseWriter, *http.Request)

// Controller Pattern of an rest/stream controller
type Controller interface {
	GetRoutes() Routes
}

// DefaultController Default implementation
type DefaultController struct {
}

// Routes Holder of all routes
type Routes []Route

// Route Describe route
type Route struct {
	Path        string
	Method      string
	HandlerFunc RadixHandlerFunc
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
