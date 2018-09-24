package utils

import (
	"errors"
	"net/http"
	"strings"

	"github.com/statoil/radix-api-go/models"
)

// RadixMiddleware The middleware beween router and radix handler functions
type RadixMiddleware struct {
	next models.RadixHandlerFunc
}

// NewRadixMiddleware Constructor for radix middleware
func NewRadixMiddleware(next models.RadixHandlerFunc) *RadixMiddleware {
	handler := &RadixMiddleware{
		next,
	}

	return handler
}

// Handle Wraps radix handler methods
func (handler *RadixMiddleware) Handle(w http.ResponseWriter, r *http.Request) {
	token, err := getBearerTokenFromHeader(r)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, err)
		return
	}

	client, radixclient := GetKubernetesClient(token)
	handler.next(client, radixclient, w, r)
}

// BearerTokenVerifyerMiddleware Will verify that the request has a bearer token
func BearerTokenVerifyerMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	_, err := getBearerTokenFromHeader(r)

	if err != nil {
		WriteError(w, r, http.StatusBadRequest, err)
		return
	}

	next(w, r)
}

// GetBearerToken Gets bearer token from request header
func getBearerTokenFromHeader(r *http.Request) (string, error) {
	authorizationHeader := r.Header.Get("authorization")
	authArr := strings.Split(authorizationHeader, " ")
	var jwtToken string

	if len(authArr) != 2 {
		// For socket connections it should be in the query
		jwtToken = GetTokenFromQuery(r)
		if jwtToken == "" {
			return "", errors.New("Authentication header is invalid: " + authorizationHeader)
		}

	} else {
		jwtToken = authArr[1]
	}

	return jwtToken, nil
}

// GetTokenFromQuery Gets token from query of the request
func GetTokenFromQuery(request *http.Request) string {
	return request.URL.Query().Get("token")
}
