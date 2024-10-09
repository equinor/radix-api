package token

import (
	"context"
	"fmt"

	"github.com/go-jose/go-jose/v4/jwt"
)

func (c *unchechedClaimsPrincipal) Validate(_ context.Context) error {
	return nil
}

type unchechedClaimsPrincipal struct {
	token       string
	claims      *jwt.Claims
	azureClaims *azureClaims
}

func (p *unchechedClaimsPrincipal) Token() string {
	return p.token
}
func (p *unchechedClaimsPrincipal) IsAuthenticated() bool {
	return true
}
func (p *unchechedClaimsPrincipal) Id() string {
	if p.azureClaims.ObjectId != "" {
		return fmt.Sprintf("oid:%s", p.azureClaims.ObjectId)
	}

	return fmt.Sprintf("sub:%s", p.claims.Subject)
}

func (p *unchechedClaimsPrincipal) Name() string {
	if p.azureClaims.Upn != "" {
		return p.azureClaims.Upn
	}

	if p.azureClaims.AppDisplayName != "" {
		return p.azureClaims.AppDisplayName
	}

	if p.azureClaims.AppId != "" {
		return p.azureClaims.AppId
	}

	if p.azureClaims.ObjectId != "" {
		return p.azureClaims.ObjectId
	}

	return p.claims.Subject
}
