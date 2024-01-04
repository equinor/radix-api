package utils

import (
	"net/http"
	"time"

	"github.com/equinor/radix-api/api/metrics"
	"github.com/equinor/radix-api/models"
	radixhttp "github.com/equinor/radix-common/net/http"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RadixMiddleware The middleware between router and radix handler functions
type RadixMiddleware struct {
	kubeUtil     KubeUtil
	path         string
	method       string
	allowNoAuth  bool
	kubeApiQPS   float32
	kubeApiBurst int
	next         models.RadixHandlerFunc
}

// NewRadixMiddleware Constructor for radix middleware
func NewRadixMiddleware(kubeUtil KubeUtil, path, method string, allowUnauthenticatedUsers bool, kubeApiQPS float32, kubeApiBurst int, next models.RadixHandlerFunc) *RadixMiddleware {
	handler := &RadixMiddleware{
		kubeUtil,
		path,
		method,
		allowUnauthenticatedUsers,
		kubeApiQPS,
		kubeApiBurst,
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
	useOutClusterClient := handler.kubeUtil.IsUseOutClusterClient()
	token, err := getBearerTokenFromHeader(r, useOutClusterClient)

	if err != nil {
		if err = radixhttp.ErrorResponse(w, r, err); err != nil {
			log.Errorf("handleAuthorization: failed to write error response: %v", err)
		}
		return
	}

	impersonation, err := radixhttp.GetImpersonationFromHeader(r)
	if err != nil {
		if err = radixhttp.ErrorResponse(w, r, radixhttp.UnexpectedError("Problems impersonating", err)); err != nil {
			log.Errorf("handleAuthorization: failed to write error response: %v", err)
		}
		return
	}

	restOptions := handler.getRestClientOptions()
	inClusterClient, inClusterRadixClient, inClusterSecretProviderClient, inClusterTektonClient := handler.kubeUtil.GetInClusterKubernetesClient(restOptions...)
	outClusterClient, outClusterRadixClient, outClusterSecretProviderClient, outClusterTektonClient := handler.kubeUtil.GetOutClusterKubernetesClientWithImpersonation(token, impersonation, restOptions...)

	accounts := models.NewAccounts(
		inClusterClient,
		inClusterRadixClient,
		inClusterSecretProviderClient,
		inClusterTektonClient,
		outClusterClient,
		outClusterRadixClient,
		outClusterSecretProviderClient,
		outClusterTektonClient,
		token,
		impersonation)

	// Check if registration of application exists for application-specific requests
	if appName, exists := mux.Vars(r)["appName"]; exists {
		if _, err := accounts.UserAccount.RadixClient.RadixV1().RadixRegistrations().Get(r.Context(), appName, metav1.GetOptions{}); err != nil {
			if err = radixhttp.ErrorResponse(w, r, err); err != nil {
				log.Errorf("handleAuthorization: failed to write error response: %v", err)
			}
			return
		}
	}

	handler.next(accounts, w, r)
}

func (handler *RadixMiddleware) getRestClientOptions() []RestClientConfigOption {
	var options []RestClientConfigOption

	if handler.kubeApiQPS > 0.0 {
		options = append(options, WithQPS(handler.kubeApiQPS))
	}

	if handler.kubeApiBurst > 0 {
		options = append(options, WithBurst(handler.kubeApiBurst))
	}

	return options
}

func (handler *RadixMiddleware) handleAnonymous(w http.ResponseWriter, r *http.Request) {
	restOptions := handler.getRestClientOptions()
	inClusterClient, inClusterRadixClient, inClusterSecretProviderClient, inClusterTektonClient := handler.kubeUtil.GetInClusterKubernetesClient(restOptions...)

	sa := models.NewServiceAccount(inClusterClient, inClusterRadixClient, inClusterSecretProviderClient, inClusterTektonClient)
	accounts := models.Accounts{ServiceAccount: sa}

	handler.next(accounts, w, r)
}

func getBearerTokenFromHeader(r *http.Request, useOutClusterClient bool) (string, error) {
	if useOutClusterClient {
		return radixhttp.GetBearerTokenFromHeader(r)
	}
	// if we're in debug mode, arbitrary bearer token is injected
	return "some_arbitrary_token", nil
}
