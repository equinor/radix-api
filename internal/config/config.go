package config

import (
	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog/log"
)

type Config struct {
	Port                        int    `envconfig:"PORT" default:"3002" desc:"Port where API will be served"`
	MetricsPort                 int    `envconfig:"METRICS_PORT" default:"9090"  desc:"Port where Metrics will be served"`
	ProfilePort                 int    `envconfig:"PROFILE_PORT" default:"7070"  desc:"Port where Profiler will be served"`
	UseProfiler                 bool   `envconfig:"USE_PROFILER" default:"false" desc:"Enable Profiler"`
	ClusterName                 string `envconfig:"RADIX_CLUSTERNAME" required:"true"`
	DNSZone                     string `envconfig:"RADIX_DNS_ZONE" required:"true"`
	OidcIssuer                  string `envconfig:"OIDC_ISSUER" required:"true"`
	OidcAudience                string `envconfig:"OIDC_AUDIENCE" required:"true"`
	KubernetesApiServer         string `envconfig:"K8S_API_HOST" default:"https://kubernetes.default.svc"`
	LogLevel                    string `envconfig:"LOG_LEVEL" default:"info"`
	LogPrettyPrint              bool   `envconfig:"LOG_PRETTY" default:"false"`
	PipelineImageTag            string `envconfig:"PIPELINE_IMG_TAG"`
	TektonImageTag              string `envconfig:"TEKTON_IMG_TAG"`
	RequireAppConfigurationItem bool   `envconfig:"REQUIRE_APP_CONFIGURATION_ITEM" default:"true"`
	RequireAppADGroups          bool   `envconfig:"REQUIRE_APP_AD_GROUPS" default:"true"`
	AppName                     string `envconfig:"RADIX_APP" required:"true"`
	EnvironmentName             string `envconfig:"RADIX_ENVIRONMENT" required:"true"`
	PrometheusUrl               string `envconfig:"PROMETHEUS_URL" required:"true"`
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
