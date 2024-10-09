package token

import (
	"context"
	"fmt"
	"net/url"

	"github.com/equinor/radix-common/net/http"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

type UncheckedValidator struct{}

var _ ValidatorInterface = &UncheckedValidator{}

func NewUncheckedValidator(_ *url.URL, _ string) (*UncheckedValidator, error) {
	return &UncheckedValidator{}, nil
}

func (v *UncheckedValidator) ValidateToken(_ context.Context, token string) (TokenPrincipal, error) {
	var registeredClaims jwt.Claims
	var azureClaims azureClaims

	jwt, err := jwt.ParseSigned(token, []jose.SignatureAlgorithm{jose.HS256, jose.RS256})
	if err != nil {
		return nil, http.ForbiddenError("invalid token")
	}
	err = jwt.UnsafeClaimsWithoutVerification(&registeredClaims, &azureClaims)
	if err != nil {
		return nil, http.ForbiddenError(fmt.Sprintf("failed to extract JWT unsafeClaims: %s", err.Error()))
	}

	principal := &unchechedClaimsPrincipal{token: token, claims: &registeredClaims, azureClaims: &azureClaims}
	return principal, nil
}
