package utils

import (
	"context"
	"net/http"
	"time"

	"github.com/equinor/radix-api/api/metrics"
	"github.com/equinor/radix-api/models"
	radixhttp "github.com/equinor/radix-common/net/http"
	"github.com/gorilla/mux"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RadixMiddleware The middleware between router and radix handler functions
type RadixMiddleware struct {
	kubeUtil    KubeUtil
	path        string
	method      string
	allowNoAuth bool
	next        models.RadixHandlerFunc
}

// NewRadixMiddleware Constructor for radix middleware
func NewRadixMiddleware(kubeUtil KubeUtil, path, method string, allowUnauthenticatedUsers bool, next models.RadixHandlerFunc) *RadixMiddleware {
	handler := &RadixMiddleware{
		kubeUtil,
		path,
		method,
		allowUnauthenticatedUsers,
		next,
	}

	return handler
}

// Handle Wraps radix handler methods
func (handler *RadixMiddleware) Handle(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	defer func() {
		httpDuration := time.Since(start)
		metrics.AddRequestDuration(handler.path, handler.method, httpDuration)
	}()

	switch {
	case handler.allowNoAuth:
		handler.handleAnonymous(w, r)
	default:
		handler.handleAuthorization(w, r)
	}
}

func (handler *RadixMiddleware) handleAuthorization(w http.ResponseWriter, r *http.Request) {
	token, err := radixhttp.GetBearerTokenFromHeader(r)

	if err != nil {
		radixhttp.ErrorResponse(w, r, err)
		return
	}

	impersonation, err := radixhttp.GetImpersonationFromHeader(r)
	if err != nil {
		radixhttp.ErrorResponse(w, r, radixhttp.UnexpectedError("Problems impersonating", err))
		return
	}

	inClusterClient, inClusterRadixClient := handler.kubeUtil.GetInClusterKubernetesClient()
	outClusterClient, outClusterRadixClient := handler.kubeUtil.GetOutClusterKubernetesClientWithImpersonation(token, impersonation)

	accounts := models.NewAccounts(
		inClusterClient,
		inClusterRadixClient,
		outClusterClient,
		outClusterRadixClient,
		token,
		impersonation)

	// Check if registration of application exists for application-specific requests
	if appName, exists := mux.Vars(r)["appName"]; exists {
		if _, err := accounts.UserAccount.RadixClient.RadixV1().RadixRegistrations().Get(context.TODO(), appName, metav1.GetOptions{}); err != nil {
			radixhttp.ErrorResponse(w, r, err)
			return
		}
	}

	handler.next(accounts, w, r)
}

func (handler *RadixMiddleware) handleAnonymous(w http.ResponseWriter, r *http.Request) {
	inClusterClient, inClusterRadixClient := handler.kubeUtil.GetInClusterKubernetesClient()

	sa := models.NewServiceAccount(inClusterClient, inClusterRadixClient)
	accounts := models.Accounts{ServiceAccount: sa}

	handler.next(accounts, w, r)
}
