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
	MachineUser bool `json:"machineUser"`
}
