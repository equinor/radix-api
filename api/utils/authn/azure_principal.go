package token

import (
	"context"

	"github.com/auth0/go-jwt-middleware/v2/validator"
)

type azureClaims struct {
	ObjectId string `json:"oid,omitempty"`
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
func (p *AzurePrincipal) Id() string { return p.azureClaims.ObjectId }
