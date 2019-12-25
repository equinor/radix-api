package models

import (
	"errors"
	"strings"
)

// Impersonation holds user and group to impersonate
type Impersonation struct {
	User  string
	Group string
}

// NewImpersonation Constructor
func NewImpersonation(user, group string) (Impersonation, error) {
	impersonation := Impersonation{
		User:  strings.TrimSpace(user),
		Group: strings.TrimSpace(group),
	}
	return impersonation, impersonation.isValid()
}

// PerformImpersonation Impersonate user
func (impersonation Impersonation) PerformImpersonation() bool {
	return impersonation.User != "" && impersonation.Group != ""
}

func (impersonation Impersonation) isValid() error {
	impersonateUserSet := impersonation.User != ""
	impersonateGroupSet := impersonation.Group != ""

	if (impersonateUserSet && !impersonateGroupSet) ||
		(!impersonateUserSet && impersonateGroupSet) {
		return errors.New("Impersonation cannot be done without both user and group being set")
	}
	return nil
}
