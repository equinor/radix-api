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
	PipelineImageTag            string `envconfig:"PIPELINE_IMG_TAG" default:"latest"`
	TektonImageTag              string `envconfig:"TEKTON_IMG_TAG" default:"release-latest"`
	RequireAppConfigurationItem bool   `envconfig:"REQUIRE_APP_CONFIGURATION_ITEM" default:"true"`
	RequireAppADGroups          bool   `envconfig:"REQUIRE_APP_AD_GROUPS" default:"true"`
	LogLevel                    string `envconfig:"LOG_LEVEL" default:"info"`
	LogPrettyPrint              bool   `envconfig:"LOG_PRETTY" default:"false"`
	ClusterName                 string `envconfig:"RADIX_CLUSTERNAME" required:"true"`
	DNSZone                     string `envconfig:"RADIX_DNS_ZONE" required:"true"`
	AzureOidc                   Oidc   `envconfig:"OIDC_AZURE" required:"true"`
	KubernetesOidc              Oidc   `envconfig:"OIDC_KUBERNETES" required:"true"`
	AppName                     string `envconfig:"RADIX_APP" required:"true"`
	EnvironmentName             string `envconfig:"RADIX_ENVIRONMENT" required:"true"`
	PrometheusUrl               string `envconfig:"PROMETHEUS_URL" required:"true"`
}

type Oidc struct {
	Issuer   IssuerUrl `envconfig:"ISSUER" required:"true"`
	Audience string    `envconfig:"Audience" required:"true"`
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

type IssuerUrl url.URL

func (ipd *IssuerUrl) Decode(value string) error {
	issuer, err := url.Parse(value)
	if err != nil {
		return err
	}

	*ipd = IssuerUrl(*issuer)
	return nil
}

func (ipd *IssuerUrl) ToURL() url.URL {
	return url.URL(*ipd)
}
