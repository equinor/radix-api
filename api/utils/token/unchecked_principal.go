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
		return fmt.Sprintf("unverified: %s", p.azureClaims.ObjectId)
	}

	return fmt.Sprintf("unverified: %s (sub!)", p.claims.Subject)
}

func (p *unchechedClaimsPrincipal) Name() string {
	if p.azureClaims.Upn != "" {
		return fmt.Sprintf("unverified: %s", p.azureClaims.Upn)
	}

	if p.azureClaims.AppDisplayName != "" {
		return fmt.Sprintf("unverified: %s", p.azureClaims.AppDisplayName)
	}

	if p.azureClaims.AppId != "" {
		return fmt.Sprintf("unverified: %s", p.azureClaims.AppId)
	}

	if p.azureClaims.ObjectId != "" {
		return fmt.Sprintf("unverified: %s", p.azureClaims.ObjectId)
	}

	return fmt.Sprintf("unverified: %s", p.claims.Subject)
}
