package token

import (
	"context"
	"errors"
	"fmt"
)

type ChainedValidator struct{ validators []ValidatorInterface }

var _ ValidatorInterface = &ChainedValidator{}
var errNoIssuersFound = errors.New("no issuers found")

func NewChainedValidator(validators ...ValidatorInterface) *ChainedValidator {
	return &ChainedValidator{validators}
}

func (v *ChainedValidator) ValidateToken(ctx context.Context, token string) (TokenPrincipal, error) {
	var errs []error

	for _, validator := range v.validators {
		principal, err := validator.ValidateToken(ctx, token)
		if principal != nil {
			return principal, nil
		}
		errs = append(errs, err)
	}

	return nil, fmt.Errorf("%w: %v", errNoIssuersFound, errors.Join(errs...))
}
