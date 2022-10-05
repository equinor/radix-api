package models

// ApplicationRegistrationPatch contains fields that can be patched on a registration
// swagger:model ApplicationRegistrationPatch
type ApplicationRegistrationPatch struct {
	// AdGroups the groups that should be able to access the application
	//
	// required: false
	AdGroups *[]string `json:"adGroups,omitempty"`

	// Owner of the application - should be an email
	//
	// required: false
	Owner *string `json:"owner,omitempty"`

	// MachineUser is used for interacting directly with Radix API
	//
	// required: false
	// Extensions:
	// x-nullable: true
	MachineUser *bool `json:"machineUser,omitempty"`

	// Repository the github repository
	//
	// required: false
	Repository *string `json:"repository,omitempty"`

	// WBS information
	//
	// required: false
	WBS *string `json:"wbs,omitempty"`

	// ConfigBranch information
	//
	// required: false
	ConfigBranch *string `json:"configBranch,omitempty"`

	// ConfigurationItem is an identifier for an entity in a configuration management solution such as a CMDB.
	// ITIL defines a CI as any component that needs to be managed in order to deliver an IT Service
	// Ref: https://en.wikipedia.org/wiki/Configuration_item
	//
	// required: false
	ConfigurationItem *string `json:"configurationItem"`
}
