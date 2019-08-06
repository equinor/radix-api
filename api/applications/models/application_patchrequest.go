package models

// ApplicationPatchRequest contains fields that can be patched on a registration
// swagger:model ApplicationPatchRequest
type ApplicationPatchRequest struct {
	// AdGroups the groups that should be able to access the application
	//
	// required: true
	AdGroups []string `json:"adGroups"`
}
