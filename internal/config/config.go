package config

import (
	"net/url"

	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog/log"
)

type Config struct {
	Port                        int    `envconfig:"PORT" default:"3002" desc:"Port where API will be served"`
	MetricsPort                 int    `envconfig:"METRICS_PORT" default:"9090"  desc:"Port where Metrics will be served"`
	ProfilePort                 int    `envconfig:"PROFILE_PORT" default:"7070"  desc:"Port where Profiler will be served"`
	UseProfiler                 bool   `envconfig:"USE_PROFILER" default:"false" desc:"Enable Profiler"`
	RequireAppConfigurationItem bool   `envconfig:"REQUIRE_APP_CONFIGURATION_ITEM" default:"true"`
	RequireAppADGroups          bool   `envconfig:"REQUIRE_APP_AD_GROUPS" default:"true"`
	LogLevel                    string `envconfig:"LOG_LEVEL" default:"info"`
	LogPrettyPrint              bool   `envconfig:"LOG_PRETTY" default:"false"`

	AppName         string `envconfig:"RADIX_APP" required:"true" desc:"Should be radix-api"`
	EnvironmentName string `envconfig:"RADIX_ENVIRONMENT" required:"true" desc:"Should be qa or prod"`
	DNSZone         string `envconfig:"RADIX_DNS_ZONE" required:"true" desc:"should be <env>.radix.equinor.com"`
	AzureOidc       Oidc   `envconfig:"OIDC_AZURE" required:"true"`
	KubernetesOidc  Oidc   `envconfig:"OIDC_KUBERNETES" required:"true"`
	PrometheusUrl   string `envconfig:"PROMETHEUS_URL" required:"true"`
}

type Oidc struct {
	Issuer   url.URL `envconfig:"ISSUER" required:"true"`
	Audience string  `envconfig:"Audience" required:"true"`
}

func MustParse() Config {
	var s Config
	err := envconfig.Process("", &s)
	if err != nil {
		_ = envconfig.Usage("", &s)
		log.Fatal().Msg(err.Error())
	}

	return s
}
