package models

// MachineUser Holds info about machine user
// swagger:model MachineUser
type MachineUser struct {
	// Token the value of the token
	//
	// required: true
	Token string `json:"token"`

	// ExpirationTimestamp the expiration timestamp of the token
	//
	// required: false
	ExpirationTimestamp string `json:"expirationTimestamp"`
	// Not required because of backward compatibility
}
