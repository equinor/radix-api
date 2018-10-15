package platform

// ApplicationRegistration describe an application
// swagger:model ApplicationRegistration
type ApplicationRegistration struct {
	// Name the unique name of the Radix application
	//
	// required: true
	// example: radix-canary-golang
	Name string `json:"name"`

	// Repository the github repository
	//
	// required: true
	// example: https://github.com/Statoil/radix-canary-golang
	Repository string `json:"repository"`

	// SharedSecret the shared secret of the webhook
	//
	// required: true
	SharedSecret string `json:"sharedSecret"`

	// AdGroups the groups that should be able to access the application
	//
	// required: true
	AdGroups []string `json:"adGroups"`

	// PublicKey the public part of the deploy key returned
	// after successful registration
	//
	// required: false
	PublicKey string `json:"publicKey,omitempty"`
}
