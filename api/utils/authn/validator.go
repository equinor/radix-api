package token

import (
	"context"
	"errors"
	"net/url"
	"time"

	"github.com/auth0/go-jwt-middleware/v2/jwks"
	"github.com/auth0/go-jwt-middleware/v2/validator"
)

type TokenPrincipal interface {
	IsAuthenticated() bool
	Token() string
	Subject() string
}

type ValidatorInterface interface {
	ValidateToken(context.Context, string) (TokenPrincipal, error)
}

type Validator struct {
	validator *validator.Validator
}

var _ ValidatorInterface = &Validator{}

func NewValidator(issuerUrl *url.URL, audience string) (*Validator, error) {
	provider := jwks.NewCachingProvider(issuerUrl, 5*time.Minute)

	validator, err := validator.New(
		provider.KeyFunc,
		validator.RS256,
		issuerUrl.String(),
		[]string{audience},
		validator.WithCustomClaims(func() validator.CustomClaims {
			return &azureClaims{}
		}),
	)
	if err != nil {
		return nil, err
	}

	return &Validator{validator: validator}, nil
}

func (v *Validator) ValidateToken(ctx context.Context, token string) (TokenPrincipal, error) {
	validateToken, err := v.validator.ValidateToken(ctx, token)
	if err != nil {
		return nil, err
	}

	claims, ok := validateToken.(*validator.ValidatedClaims)
	if !ok {
		return nil, errors.New("invalid token")
	}

	azureClaims, ok := claims.CustomClaims.(*azureClaims)
	if !ok {
		return nil, errors.New("invalid azure token")
	}

	principal := &AzurePrincipal{token: token, claims: claims, azureClaims: azureClaims}
	return principal, nil
}
