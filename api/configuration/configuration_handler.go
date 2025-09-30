package configuration

import (
	"context"

	configurationModels "github.com/equinor/radix-api/api/configuration/models"
	"github.com/equinor/radix-api/models"
)

type configurationHandler struct {
	accounts models.Accounts
}

type ConfigurationHandler interface {
	// Init Constructor
	GetSettings(ctx context.Context) (configurationModels.Settings, error)
}

// Init Constructor
func Init(accounts models.Accounts) ConfigurationHandler {
	return &configurationHandler{
		accounts: accounts,
	}
}

func (h *configurationHandler) GetSettings(ctx context.Context) (configurationModels.Settings, error) {
	return configurationModels.Settings{
		ClusterEgressIps:   []string{"104.45.84.0/30"},
		ClusterOidcIssuers: []string{"https://login.microsoftonline.com/72f988bf-86f1-41af-91ab-2d7cd011db47/v2.0"},
		ClusterBaseDomain:  "dev.radix.equinor.com",
		ClusterType:        "development",
		ClusterName:        "weekly-40",
	}, nil
}
