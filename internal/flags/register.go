package flags

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func Register(v *viper.Viper, prefix string, flagSet *pflag.FlagSet, config interface{}) error {
	val := reflect.ValueOf(config)
	var typ reflect.Type
	if val.Kind() == reflect.Ptr {
		typ = val.Elem().Type()
	} else {
		typ = val.Type()
	}

	for i := 0; i < typ.NumField(); i++ {
		// pull out the struct tags:
		//    flag - the name of the command line flag
		//    cfg - the name of the config file option
		field := typ.Field(i)
		fieldV := reflect.Indirect(val).Field(i)
		fieldName := strings.Join([]string{prefix, field.Name}, ".")

		cfgName := field.Tag.Get("cfg")
		if cfgName == ",internal" {
			// Public but internal types that should not be exposed to users, skip them
			continue
		}

		if field.Name == strings.ToLower(field.Name) {
			// Unexported fields cannot be set by a user, so won't have tags or flags, skip them
			continue
		}

		if field.Type.Kind() == reflect.Struct {
			if cfgName != ",squash" {
				return fmt.Errorf("field %q does not have required cfg tag: `,squash`", fieldName)
			}
			err := Register(v, fieldName, flagSet, fieldV.Interface())
			if err != nil {
				return err
			}
			continue
		}

		flagName := field.Tag.Get("flag")
		if flagName == "" || cfgName == "" {
			return fmt.Errorf("field %q does not have required tags (cfg, flag)", fieldName)
		}

		if flagSet == nil {
			return fmt.Errorf("flagset cannot be nil")
		}

		f := flagSet.Lookup(flagName)
		if f == nil {
			return fmt.Errorf("field %q does not have a registered flag", flagName)
		}
		err := v.BindPFlag(cfgName, f)
		if err != nil {
			return fmt.Errorf("error binding flag for field %q: %w", fieldName, err)
		}
	}

	return nil
}
