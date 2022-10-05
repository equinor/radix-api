package applications

import (
	"strings"

	applicationModels "github.com/equinor/radix-api/api/applications/models"
	"github.com/equinor/radix-api/api/utils"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
)

// Builder Handles construction of DTO
type Builder interface {
	withName(name string) Builder
	withOwner(owner string) Builder
	withCreator(creator string) Builder
	withRepository(string) Builder
	withSharedSecret(string) Builder
	withAdGroups([]string) Builder
	withPublicKey(string) Builder
	withPrivateKey(string) Builder
	withCloneURL(string) Builder
	withDeployKey(*utils.DeployKey) Builder
	withMachineUser(bool) Builder
	withWBS(string) Builder
	withConfigBranch(string) Builder
	withConfigurationItem(string) Builder
	withAcknowledgeWarnings() Builder
	withAppRegistration(*applicationModels.ApplicationRegistration) Builder
	withRadixRegistration(*v1.RadixRegistration) Builder
	Build() applicationModels.ApplicationRegistration
	BuildRR() (*v1.RadixRegistration, error)
	BuildApplicationRegistrationRequest() *applicationModels.ApplicationRegistrationRequest
}

type applicationBuilder struct {
	name                string
	owner               string
	creator             string
	repository          string
	sharedSecret        string
	adGroups            []string
	publicKey           string
	privateKey          string
	cloneURL            string
	machineUser         bool
	wbs                 string
	configBranch        string
	configurationItem   string
	acknowledgeWarnings bool
}

func (rb *applicationBuilder) withAppRegistration(appRegistration *applicationModels.ApplicationRegistration) Builder {
	rb.withName(appRegistration.Name)
	rb.withRepository(appRegistration.Repository)
	rb.withSharedSecret(appRegistration.SharedSecret)
	rb.withAdGroups(appRegistration.AdGroups)
	rb.withPublicKey(appRegistration.PublicKey)
	rb.withPrivateKey(appRegistration.PrivateKey)
	rb.withOwner(appRegistration.Owner)
	rb.withWBS(appRegistration.WBS)
	rb.withConfigBranch(appRegistration.ConfigBranch)
	rb.withConfigurationItem(appRegistration.ConfigurationItem)
	return rb
}

func (rb *applicationBuilder) withRadixRegistration(radixRegistration *v1.RadixRegistration) Builder {
	rb.withName(radixRegistration.Name)
	rb.withCloneURL(radixRegistration.Spec.CloneURL)
	rb.withSharedSecret(radixRegistration.Spec.SharedSecret)
	rb.withAdGroups(radixRegistration.Spec.AdGroups)
	rb.withPublicKey(radixRegistration.Spec.DeployKeyPublic)
	rb.withOwner(radixRegistration.Spec.Owner)
	rb.withCreator(radixRegistration.Spec.Creator)
	rb.withMachineUser(radixRegistration.Spec.MachineUser)
	rb.withWBS(radixRegistration.Spec.WBS)
	rb.withConfigBranch(radixRegistration.Spec.ConfigBranch)
	rb.withConfigurationItem(radixRegistration.Spec.ConfigurationItem)

	// Private part of key should never be returned
	return rb
}

func (rb *applicationBuilder) withName(name string) Builder {
	rb.name = name
	return rb
}

func (rb *applicationBuilder) withOwner(owner string) Builder {
	rb.owner = owner
	return rb
}

func (rb *applicationBuilder) withCreator(creator string) Builder {
	rb.creator = creator
	return rb
}

func (rb *applicationBuilder) withRepository(repository string) Builder {
	rb.repository = repository
	return rb
}

func (rb *applicationBuilder) withCloneURL(cloneURL string) Builder {
	rb.cloneURL = cloneURL
	return rb
}

func (rb *applicationBuilder) withSharedSecret(sharedSecret string) Builder {
	rb.sharedSecret = sharedSecret
	return rb
}

func (rb *applicationBuilder) withAdGroups(adGroups []string) Builder {
	rb.adGroups = adGroups
	return rb
}

func (rb *applicationBuilder) withPublicKey(publicKey string) Builder {
	rb.publicKey = strings.TrimSuffix(publicKey, "\n")
	return rb
}

func (rb *applicationBuilder) withPrivateKey(privateKey string) Builder {
	rb.privateKey = strings.TrimSuffix(privateKey, "\n")
	return rb
}

func (rb *applicationBuilder) withDeployKey(deploykey *utils.DeployKey) Builder {
	if deploykey != nil {
		rb.publicKey = deploykey.PublicKey
		rb.privateKey = deploykey.PrivateKey
	}

	return rb
}

func (rb *applicationBuilder) withMachineUser(machineUser bool) Builder {
	rb.machineUser = machineUser
	return rb
}

func (rb *applicationBuilder) withWBS(wbs string) Builder {
	rb.wbs = wbs
	return rb
}

func (rb *applicationBuilder) withConfigBranch(configBranch string) Builder {
	rb.configBranch = configBranch
	return rb
}

func (rb *applicationBuilder) withConfigurationItem(ci string) Builder {
	rb.configurationItem = ci
	return rb
}

func (rb *applicationBuilder) withAcknowledgeWarnings() Builder {
	rb.acknowledgeWarnings = true
	return rb
}

func (rb *applicationBuilder) Build() applicationModels.ApplicationRegistration {
	repository := rb.repository
	if repository == "" {
		repository = crdUtils.GetGithubRepositoryURLFromCloneURL(rb.cloneURL)
	}

	return applicationModels.ApplicationRegistration{
		Name:              rb.name,
		Repository:        repository,
		SharedSecret:      rb.sharedSecret,
		AdGroups:          rb.adGroups,
		PublicKey:         rb.publicKey,
		PrivateKey:        rb.privateKey,
		Owner:             rb.owner,
		Creator:           rb.creator,
		MachineUser:       rb.machineUser,
		WBS:               rb.wbs,
		ConfigBranch:      rb.configBranch,
		ConfigurationItem: rb.configurationItem,
	}
}

func (rb *applicationBuilder) BuildRR() (*v1.RadixRegistration, error) {
	builder := crdUtils.NewRegistrationBuilder()

	radixRegistration := builder.
		WithPublicKey(rb.publicKey).
		WithPrivateKey(rb.privateKey).
		WithName(rb.name).
		WithRepository(rb.repository).
		WithSharedSecret(rb.sharedSecret).
		WithAdGroups(rb.adGroups).
		WithOwner(rb.owner).
		WithCreator(rb.creator).
		WithMachineUser(rb.machineUser).
		WithWBS(rb.wbs).
		WithConfigBranch(rb.configBranch).
		WithConfigurationItem(rb.configurationItem).
		BuildRR()

	return radixRegistration, nil
}

func (rb *applicationBuilder) BuildApplicationRegistrationRequest() *applicationModels.ApplicationRegistrationRequest {
	applicationRegistration := rb.Build()
	return &applicationModels.ApplicationRegistrationRequest{
		ApplicationRegistration: &applicationRegistration,
		AcknowledgeWarnings:     rb.acknowledgeWarnings,
	}
}

// NewBuilder Constructor for application builder
func NewBuilder() Builder {
	return &applicationBuilder{}
}

// AnApplicationRegistration Constructor for application builder with test values
func AnApplicationRegistration() Builder {
	return &applicationBuilder{
		name:       "my-app",
		repository: "https://github.com/Equinor/my-app",
		// file deepcode ignore HardcodedPassword: only used by unit test
		sharedSecret: "AnySharedSecret",
		adGroups:     []string{"a6a3b81b-34gd-sfsf-saf2-7986371ea35f"},
		owner:        "a_test_user@equinor.com",
		creator:      "a_test_user@equinor.com",
		wbs:          "T.O123A.AZ.45678",
		configBranch: "main",
	}
}
