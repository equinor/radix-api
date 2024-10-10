package token

import (
	"context"
	"errors"
)

type ChainedValidator struct{ validators []ValidatorInterface }

var _ ValidatorInterface = &ChainedValidator{}
var errNoIssuersFound = errors.New("no issuers found")

func NewChainedValidator(validators ...ValidatorInterface) *ChainedValidator {
	return &ChainedValidator{validators}
}

func (v *ChainedValidator) ValidateToken(ctx context.Context, token string) (principal TokenPrincipal, err error) {
	for index, validator := range v.validators {
		principal, err = validator.ValidateToken(ctx, token)
		if err == nil {
			return principal, nil
		} else if index == len(v.validators)-1 {
			return nil, err
		}
	}

	return nil, errNoIssuersFound
}
