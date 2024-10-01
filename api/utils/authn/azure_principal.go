package token

import (
	"context"

	"github.com/auth0/go-jwt-middleware/v2/validator"
)

type azureClaims struct {
	AppId string `json:"appid,omitempty"`
	Upn   string `json:"upn,omitempty"`
}

func (c *azureClaims) Validate(_ context.Context) error {
	return nil
}

type AzurePrincipal struct {
	token       string
	claims      *validator.ValidatedClaims
	azureClaims *azureClaims
}

func (p *AzurePrincipal) Token() string {
	return p.token
}
func (p *AzurePrincipal) IsAuthenticated() bool {
	return true
}
func (p *AzurePrincipal) Name() string {
	if p.azureClaims.Upn != "" {
		return p.azureClaims.Upn
	}

	if p.azureClaims.AppId != "" {
		return p.azureClaims.AppId
	}

	return p.claims.RegisteredClaims.Subject
}
