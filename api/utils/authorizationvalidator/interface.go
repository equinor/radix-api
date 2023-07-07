package authorizationvalidator

import (
	"context"
	"github.com/equinor/radix-api/models"
)

type Interface interface {
	// UserIsAdmin checks if user is admin
	UserIsAdmin(ctx context.Context, user *models.Account, appName string) (bool, error)
}
