package configuration

import (
	"context"

	configurationModels "github.com/equinor/radix-api/api/configuration/models"
	"github.com/equinor/radix-api/internal/config"
)

type configurationHandler struct {
	config config.Config
}

type ConfigurationHandler interface {
	// Init Constructor
	GetSettings(ctx context.Context) (configurationModels.Settings, error)
}

// Init Constructor
func Init(config config.Config) ConfigurationHandler {
	return &configurationHandler{
		config: config,
	}
}

func (h *configurationHandler) GetSettings(ctx context.Context) (configurationModels.Settings, error) {
	return configurationModels.Settings{
		ClusterEgressIps:   h.config.ClusterEgressIps,
		ClusterOidcIssuers: h.config.ClusterOidcIssuers,
		DNSZone:            h.config.DNSZone,
		ClusterName:        h.config.ClusterName,
	}, nil
}
