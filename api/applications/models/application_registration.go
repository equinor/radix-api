package models

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
	// example: https://github.com/equinor/radix-canary-golang
	Repository string `json:"repository"`

	// SharedSecret the shared secret of the webhook
	//
	// required: true
	SharedSecret string `json:"sharedSecret"`

	// AdGroups the groups that should be able to access the application
	//
	// required: true
	AdGroups []string `json:"adGroups"`

	// Owner of the application (email). Can be a single person or a shared group email
	//
	// required: true
	Owner string `json:"owner"`

	// Owner of the application (email). Can be a single person or a shared group email
	//
	// required: true
	Creator string `json:"creator"`

	// PublicKey the public part of the deploy key set or returned
	// after successful application
	//
	// required: false
	PublicKey string `json:"publicKey,omitempty"`

	// PrivateKey the private part of the deploy key set or returned
	// after successful application
	//
	// required: false
	PrivateKey string `json:"privateKey,omitempty"`
}
