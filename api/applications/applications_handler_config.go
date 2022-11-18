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
	RequireAppConfigurationItem bool `cfg:"require_app_configuration_item" flag:"require-app-configuration-item"`
}

func ApplicationHandlerConfigFlagSet() *pflag.FlagSet {
	flagset := pflag.NewFlagSet("config", pflag.ExitOnError)
	flagset.ParseErrorsWhitelist = pflag.ParseErrorsWhitelist{UnknownFlags: true}
	flagset.Bool("require-app-configuration-item", true, "Require configuration item for application")
	return flagset
}
