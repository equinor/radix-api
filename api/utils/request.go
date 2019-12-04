package utils

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/equinor/radix-api/models"
)

// GetBearerToken Gets bearer token from request header
func getBearerTokenFromHeader(r *http.Request) (string, error) {
	authorizationHeader := r.Header.Get("authorization")
	authArr := strings.Split(authorizationHeader, " ")
	var jwtToken string

	if len(authArr) != 2 {
		return "", errors.New("Authentication header is invalid: " + authorizationHeader)
	}

	jwtToken = authArr[1]
	return jwtToken, nil
}

// GetBearerToken Gets bearer token from request header
func getImpersonationFromHeader(r *http.Request) (models.Impersonation, error) {
	impersonateUser := r.Header.Get("Impersonate-User")
	impersonateGroup := r.Header.Get("Impersonate-Group")
	return models.NewImpersonation(impersonateUser, impersonateGroup)
}

// GetTokenFromQuery Gets token from query of the request
func GetTokenFromQuery(request *http.Request) string {
	return request.URL.Query().Get("token")
}

// IsWatch Indicates if request is asking for watch
func isWatch(request *http.Request) (bool, error) {
	watchArg := request.FormValue("watch")
	var watch bool
	if watchArg != "" {
		parsedWatchArg, err := strconv.ParseBool(watchArg)

		if err != nil {
			return false, err
		}

		watch = parsedWatchArg
	}

	return watch, nil
}
