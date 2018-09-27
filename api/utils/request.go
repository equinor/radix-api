package utils

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
)

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
