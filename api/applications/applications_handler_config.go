package applications

import (
	"github.com/equinor/radix-api/internal/flags"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func LoadApplicationHandlerConfig(args []string) (ApplicationHandlerConfig, error) {
	var cfg ApplicationHandlerConfig

	flagset := ApplicationHandlerConfigFlagSet()
	if err := flagset.Parse(args); err != nil {
		return cfg, err
	}

	v := viper.New()
	v.AutomaticEnv()

	if err := flags.Register(v, "", flagset, &cfg); err != nil {
		return cfg, err
	}

	err := v.UnmarshalExact(&cfg, func(dc *mapstructure.DecoderConfig) { dc.TagName = "cfg" })
	return cfg, err
}

type ApplicationHandlerConfig struct {
	RequireAppConfigurationItem bool   `cfg:"require_app_configuration_item" flag:"require-app-configuration-item"`
	RequireAppADGroups          bool   `cfg:"require_app_ad_groups" flag:"require-app-ad-groups"`
	AppName                     string `cfg:"radix_app" flag:"radix-app"`
	EnvironmentName             string `cfg:"radix_environment" flag:"radix-environment"`
	DNSZone                     string `cfg:"radix_dns_zone" flag:"radix-dns-zone"`
	PrometheusUrl               string `cfg:"prometheus_url" flag:"prometheus-url"`
}

func ApplicationHandlerConfigFlagSet() *pflag.FlagSet {
	flagset := pflag.NewFlagSet("config", pflag.ExitOnError)
	flagset.ParseErrorsWhitelist = pflag.ParseErrorsWhitelist{UnknownFlags: true}
	flagset.Bool("require-app-configuration-item", true, "Require configuration item for application")
	flagset.Bool("require-app-ad-groups", true, "Require AD groups for application")
	flagset.String("radix-app", "", "Application name")
	flagset.String("radix-environment", "", "Environment name")
	flagset.String("radix-dns-zone", "", "Radix DNS zone")
	flagset.String("prometheus-url", "", "Prometheus URL")
	return flagset
}
