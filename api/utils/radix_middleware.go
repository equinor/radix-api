package utils

import (
	"errors"
	"net/http"

	"github.com/statoil/radix-api/models"
)

// RadixMiddleware The middleware beween router and radix handler functions
type RadixMiddleware struct {
	kubeUtil KubeUtil
	next     models.RadixHandlerFunc
}

// NewRadixMiddleware Constructor for radix middleware
func NewRadixMiddleware(kubeUtil KubeUtil, next models.RadixHandlerFunc) *RadixMiddleware {
	handler := &RadixMiddleware{
		kubeUtil,
		next,
	}

	return handler
}

// Handle Wraps radix handler methods
func (handler *RadixMiddleware) Handle(w http.ResponseWriter, r *http.Request) {
	token, err := getBearerTokenFromHeader(r)
	if err != nil {
		ErrorResponse(w, r, err)
		return
	}

	client, radixclient := handler.kubeUtil.GetOutClusterKubernetesClient(token)
	handler.next(client, radixclient, w, r)
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
