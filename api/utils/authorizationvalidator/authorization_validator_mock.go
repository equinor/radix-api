package authorizationvalidator

import (
	"context"
	"github.com/equinor/radix-api/models"
)

var mockedValidator = mockValidator{}

func MockAuthorizationValidator() Interface {
	return &mockedValidator
}

type mockValidator struct{}

func (v *mockValidator) UserIsAdmin(ctx context.Context, user *models.Account, appName string) (bool, error) {
	return true, nil
}
