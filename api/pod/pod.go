package pod

// Pod hold info about pod
// swagger:model pod
type Pod struct {
	// Name of the pod
	//
	// required: true
	Name string `json:"name"`
}
