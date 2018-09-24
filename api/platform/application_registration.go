package platform

// ApplicationRegistration describe an application
// swagger:model applicationRegistration
type ApplicationRegistration struct {
	// Repository the github repository
	//
	// required: true
	Repository string `json:"repository"`

	// SharedSecret the shared secret of the webhook
	//
	// required: true
	SharedSecret string `json:"sharedSecret"`

	// AdGroups the groups that should be able to access the application
	//
	// required: true
	AdGroups []string `json:"adGroups"`

	// AdGroups the public part of the deploy key
	//
	// required: true
	PublicKey string `json:"publicKey,omitempty"`
}
