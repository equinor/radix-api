package token

import (
	"context"
	"errors"

	"github.com/auth0/go-jwt-middleware/v2/validator"
)

type azureClaims struct {
	AppId string `json:"appid,omitempty"`
	Upn   string `json:"upn,omitempty"`
}

func (c *azureClaims) Validate(_ context.Context) error {
	if c == nil {
		return errors.New("invalid claim")
	}

	if c.Upn == "" && c.AppId == "" {
		return errors.New("missing one of the required field: upn,appid")
	}

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
func (p *AzurePrincipal) Subject() string {
	if p.azureClaims.Upn != "" {
		return p.azureClaims.Upn
	}

	return p.azureClaims.AppId
}
