package applications

import (
	"github.com/equinor/radix-api/api/utils/authorizationvalidator"
	"github.com/equinor/radix-api/models"
)

// ApplicationHandlerFactory defines a factory function for creating an ApplicationHandler
type ApplicationHandlerFactory func(accounts models.Accounts) ApplicationHandler

// NewApplicationHandlerFactory creates a new ApplicationHandlerFactory
func NewApplicationHandlerFactory(config ApplicationHandlerConfig, authorizationValidator authorizationvalidator.Interface) ApplicationHandlerFactory {
	return func(accounts models.Accounts) ApplicationHandler {
		return NewApplicationHandler(accounts, config, authorizationValidator)
	}
}
