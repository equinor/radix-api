package deployment

// PromotionParameters describe environment to promote from and to
// swagger:model PromotionParameters
type PromotionParameters struct {
	// ImageTag optional image tag to promote from
	//
	// required: false
	// example: for radixdev.azurecr.io/radix-static-html-app:tzbqi it would be tzbqi
	ImageTag string `json:"imageTag"`

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
