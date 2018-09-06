package types

// RadixApplication describe an application
type ApplicationRegistration struct {
	Repository   string   `json:"repository"`
	SharedSecret string   `json:sharedSecret`
	AdGroups     []string `json:"adGroups"`
	PublicKey    string   `json:"publicKey,omitempty"`
}
