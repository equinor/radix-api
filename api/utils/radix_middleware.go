package utils

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/equinor/radix-api/api/metrics"
	"github.com/equinor/radix-api/models"
	"github.com/gorilla/mux"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RadixMiddleware The middleware beween router and radix handler functions
type RadixMiddleware struct {
	kubeUtil KubeUtil
	path     string
	method   string
	next     models.RadixHandlerFunc
}

// NewRadixMiddleware Constructor for radix middleware
func NewRadixMiddleware(kubeUtil KubeUtil, path, method string, next models.RadixHandlerFunc) *RadixMiddleware {
	handler := &RadixMiddleware{
		kubeUtil,
		path,
		method,
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

	token, err := getBearerTokenFromHeader(r)
	if err != nil {
		ErrorResponse(w, r, err)
		return
	}

	impersonateUser, impersonateGroup := getImpersonationFromHeader(r)

	inClusterClient, inClusterRadixClient := handler.kubeUtil.GetInClusterKubernetesClient()
	outClusterClient, outClusterRadixClient := handler.kubeUtil.GetOutClusterKubernetesClientWithImpersonation(token, impersonateUser, impersonateGroup)

	clients := models.Clients{
		InClusterClient:       inClusterClient,
		InClusterRadixClient:  inClusterRadixClient,
		OutClusterClient:      outClusterClient,
		OutClusterRadixClient: outClusterRadixClient,
	}

	// Check if registration of application exists for application-specific requests
	if appName, exists := mux.Vars(r)["appName"]; exists {
		if _, err := clients.OutClusterRadixClient.RadixV1().RadixRegistrations().Get(appName, metav1.GetOptions{}); err != nil {
			ErrorResponse(w, r, TypeMissingError(fmt.Sprintf("Unable to get registration for app %s", appName), err))
			return
		}
	}

	handler.next(clients, w, r)
}

// BearerTokenHeaderVerifyerMiddleware Will verify that the request has a bearer token in header
func BearerTokenHeaderVerifyerMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	_, err := getBearerTokenFromHeader(r)

	if err != nil {
		ErrorResponse(w, r, err)
		return
	}

	next(w, r)
}

// BearerTokenQueryVerifyerMiddleware Will verify that the request has a bearer token as query variable
func BearerTokenQueryVerifyerMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	// For socket connections it should be in the query
	jwtToken := GetTokenFromQuery(r)
	if jwtToken == "" {
		ErrorResponse(w, r, errors.New("Authentication token is required"))
		return
	}

	next(w, r)
}
