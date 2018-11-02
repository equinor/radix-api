package models

// PromotionParameters describe environment to promote from and to
// swagger:model PromotionParameters
type PromotionParameters struct {
	// FromEnvironment the environment to promote from
	//
	// required: true
	// example: dev
	FromEnvironment string `json:"fromEnvironment"`

	// ToEnvironment the environment to promote to
	//
	// required: true
	// example: prod
	ToEnvironment string `json:"toEnvironment"`
}
