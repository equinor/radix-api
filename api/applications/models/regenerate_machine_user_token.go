package models

// RegenerateMachineUserToken Holds number of days before token expires
// swagger:model RegenerateMachineUserToken
type RegenerateMachineUserToken struct {
	// DaysUntilExpiry of the machine user token
	//
	// required: true
	DaysUntilExpiry int `json:"daysUntilExpiry"`
}
