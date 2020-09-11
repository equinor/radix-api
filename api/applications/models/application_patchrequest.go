package models

// ApplicationPatchRequest contains fields that can be patched on a registration
// swagger:model ApplicationPatchRequest
type ApplicationPatchRequest struct {
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
	MachineUser *bool `json:"machineUser,omitempty"`

	// Repository the github repository
	//
	// required: false
	Repository *string `json:"repository,omitempty"`

	// WBS information
	//
	// required: false
	WBS *string `json:"wbs,omitempty"`
}
