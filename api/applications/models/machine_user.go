package models

// MachineUser Holds info about machine user
// swagger:model MachineUser
type MachineUser struct {
	// Token the value of the token
	//
	// required: true
	Token string `json:"token"`

	// TokenCreated timestamp of last generation
	//
	// example: 2006-01-02T15:04:05Z
	TokenCreated string `json:"tokenCreated"`
}
