package models

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
)

// ApplicationRegistrationBuilder Handles construction of DTO
type ApplicationRegistrationBuilder interface {
	WithName(name string) ApplicationRegistrationBuilder
	WithOwner(owner string) ApplicationRegistrationBuilder
	WithCreator(creator string) ApplicationRegistrationBuilder
	WithRepository(string) ApplicationRegistrationBuilder
	WithSharedSecret(string) ApplicationRegistrationBuilder
	WithAdGroups([]string) ApplicationRegistrationBuilder
	WithReaderAdGroups([]string) ApplicationRegistrationBuilder
	WithCloneURL(string) ApplicationRegistrationBuilder
	WithMachineUser(bool) ApplicationRegistrationBuilder
	WithWBS(string) ApplicationRegistrationBuilder
	WithConfigBranch(string) ApplicationRegistrationBuilder
	WithConfigurationItem(string) ApplicationRegistrationBuilder
	WithRadixConfigFullName(string) ApplicationRegistrationBuilder
	WithAppRegistration(*ApplicationRegistration) ApplicationRegistrationBuilder
	WithRadixRegistration(*v1.RadixRegistration) ApplicationRegistrationBuilder
	Build() ApplicationRegistration
	BuildRR() (*v1.RadixRegistration, error)
}

type applicationBuilder struct {
	name                string
	owner               string
	creator             string
	repository          string
	sharedSecret        string
	adGroups            []string
	readerAdGroups      []string
	cloneURL            string
	machineUser         bool
	wbs                 string
	configBranch        string
	configurationItem   string
	radixConfigFullName string
}

func (rb *applicationBuilder) WithAppRegistration(appRegistration *ApplicationRegistration) ApplicationRegistrationBuilder {
	rb.WithName(appRegistration.Name)
	rb.WithRepository(appRegistration.Repository)
	rb.WithSharedSecret(appRegistration.SharedSecret)
	rb.WithAdGroups(appRegistration.AdGroups)
	rb.WithReaderAdGroups(appRegistration.ReaderAdGroups)
	rb.WithOwner(appRegistration.Owner)
	rb.WithWBS(appRegistration.WBS)
	rb.WithConfigBranch(appRegistration.ConfigBranch)
	rb.WithRadixConfigFullName(appRegistration.RadixConfigFullName)
	rb.WithConfigurationItem(appRegistration.ConfigurationItem)
	return rb
}

func (rb *applicationBuilder) WithRadixRegistration(radixRegistration *v1.RadixRegistration) ApplicationRegistrationBuilder {
	rb.WithName(radixRegistration.Name)
	rb.WithCloneURL(radixRegistration.Spec.CloneURL)
	rb.WithSharedSecret(radixRegistration.Spec.SharedSecret)
	rb.WithAdGroups(radixRegistration.Spec.AdGroups)
	rb.WithReaderAdGroups(radixRegistration.Spec.ReaderAdGroups)
	rb.WithOwner(radixRegistration.Spec.Owner)
	rb.WithCreator(radixRegistration.Spec.Creator)
	rb.WithMachineUser(radixRegistration.Spec.MachineUser)
	rb.WithWBS(radixRegistration.Spec.WBS)
	rb.WithConfigBranch(radixRegistration.Spec.ConfigBranch)
	rb.WithRadixConfigFullName(radixRegistration.Spec.RadixConfigFullName)
	rb.WithConfigurationItem(radixRegistration.Spec.ConfigurationItem)

	// Private part of key should never be returned
	return rb
}

func (rb *applicationBuilder) WithName(name string) ApplicationRegistrationBuilder {
	rb.name = name
	return rb
}

func (rb *applicationBuilder) WithOwner(owner string) ApplicationRegistrationBuilder {
	rb.owner = owner
	return rb
}

func (rb *applicationBuilder) WithCreator(creator string) ApplicationRegistrationBuilder {
	rb.creator = creator
	return rb
}

func (rb *applicationBuilder) WithRepository(repository string) ApplicationRegistrationBuilder {
	rb.repository = repository
	return rb
}

func (rb *applicationBuilder) WithCloneURL(cloneURL string) ApplicationRegistrationBuilder {
	rb.cloneURL = cloneURL
	return rb
}

func (rb *applicationBuilder) WithSharedSecret(sharedSecret string) ApplicationRegistrationBuilder {
	rb.sharedSecret = sharedSecret
	return rb
}

func (rb *applicationBuilder) WithAdGroups(adGroups []string) ApplicationRegistrationBuilder {
	rb.adGroups = adGroups
	return rb
}

func (rb *applicationBuilder) WithReaderAdGroups(readerAdGroups []string) ApplicationRegistrationBuilder {
	rb.readerAdGroups = readerAdGroups
	return rb
}

func (rb *applicationBuilder) WithMachineUser(machineUser bool) ApplicationRegistrationBuilder {
	rb.machineUser = machineUser
	return rb
}

func (rb *applicationBuilder) WithWBS(wbs string) ApplicationRegistrationBuilder {
	rb.wbs = wbs
	return rb
}

func (rb *applicationBuilder) WithConfigBranch(configBranch string) ApplicationRegistrationBuilder {
	rb.configBranch = configBranch
	return rb
}

func (rb *applicationBuilder) WithConfigurationItem(ci string) ApplicationRegistrationBuilder {
	rb.configurationItem = ci
	return rb
}

func (rb *applicationBuilder) WithRadixConfigFullName(fullName string) ApplicationRegistrationBuilder {
	rb.radixConfigFullName = fullName
	return rb
}

func (rb *applicationBuilder) Build() ApplicationRegistration {
	repository := rb.repository
	if repository == "" {
		repository = crdUtils.GetGithubRepositoryURLFromCloneURL(rb.cloneURL)
	}

	return ApplicationRegistration{
		Name:                rb.name,
		Repository:          repository,
		SharedSecret:        rb.sharedSecret,
		AdGroups:            rb.adGroups,
		ReaderAdGroups:      rb.readerAdGroups,
		Owner:               rb.owner,
		Creator:             rb.creator,
		MachineUser:         rb.machineUser,
		WBS:                 rb.wbs,
		ConfigBranch:        rb.configBranch,
		RadixConfigFullName: rb.radixConfigFullName,
		ConfigurationItem:   rb.configurationItem,
	}
}

func (rb *applicationBuilder) BuildRR() (*v1.RadixRegistration, error) {
	builder := crdUtils.NewRegistrationBuilder()

	radixRegistration := builder.
		WithName(rb.name).
		WithRepository(rb.repository).
		WithSharedSecret(rb.sharedSecret).
		WithAdGroups(rb.adGroups).
		WithReaderAdGroups(rb.readerAdGroups).
		WithOwner(rb.owner).
		WithCreator(rb.creator).
		WithMachineUser(rb.machineUser).
		WithWBS(rb.wbs).
		WithConfigBranch(rb.configBranch).
		WithRadixConfigFullName(rb.radixConfigFullName).
		WithConfigurationItem(rb.configurationItem).
		BuildRR()

	return radixRegistration, nil
}

// NewApplicationRegistrationBuilder Constructor for application builder
func NewApplicationRegistrationBuilder() ApplicationRegistrationBuilder {
	return &applicationBuilder{}
}
