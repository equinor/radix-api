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

	// ConfigurationItem information
	//
	// required: false
	ConfigurationItem *string `json:"configurationItem"`
}
