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

type azurePrincipal struct {
	token       string
	claims      *validator.ValidatedClaims
	azureClaims *azureClaims
}

func (p *azurePrincipal) Token() string {
	return p.token
}
func (p *azurePrincipal) IsAuthenticated() bool {
	return true
}
func (p *azurePrincipal) Subject() string {
	if p.azureClaims.Upn != "" {
		return p.azureClaims.Upn
	}

	return p.azureClaims.AppId
}
