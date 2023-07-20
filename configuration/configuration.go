// Package configuration defines a configuration engine for the entire app.
//
// The configuration features:
//   - reads the command line arguments for the app such as authentication enabled or not.
//   - automatically loads the environment variables files.
//   - allows setting default variables if user didn't define them.
package configuration

import (
	"fmt"
	"github.com/ahmetson/common-lib/data_type/key_value"

	"github.com/ahmetson/service-lib/configuration/argument"
	"github.com/ahmetson/service-lib/configuration/env"
	"github.com/ahmetson/service-lib/log"
	"github.com/spf13/viper"
)

// Config Configuration Engine based on viper.Viper
type Config struct {
	viper *viper.Viper // used to keep default values

	Secure        bool        // Passed as --secure command line argument. If its passed then authentication is switched off.
	DebugSecurity bool        // Passed as --debug-security command line argument. If true then app prints the security logs.
	logger        *log.Logger // debug purpose only
	Service       *Service
}

// New creates a global configuration for the entire application.
//
// Automatically reads the command line arguments.
// Loads the environment variables.
func New(logger log.Logger) (*Config, error) {
	logger.Info("Reading command line arguments for application parameters")

	// First we check the parameters of the application arguments
	arguments := argument.GetArguments(&logger)

	conf := Config{
		Secure:        argument.Has(arguments, argument.Secure),
		DebugSecurity: argument.Has(arguments, argument.SecurityDebug),
		logger:        &logger,
		Service:       nil,
	}
	logger.Info("Loading environment files passed as app arguments")

	// First we load the environment variables
	err := env.LoadAnyEnv()
	if err != nil {
		return nil, fmt.Errorf("loading environment variables: %w", err)
	}

	logger.Info("Starting Viper with environment variables")

	// replace the values with the ones we fetched from environment variables
	conf.viper = viper.New()
	conf.viper.AutomaticEnv()

	conf.viper.SetDefault("SERVICE_CONFIG_NAME", "service")
	conf.viper.SetDefault("SERVICE_CONFIG_PATH", ".")

	conf.viper.SetConfigName(conf.viper.GetString("SERVICE_CONFIG_NAME"))
	conf.viper.SetConfigType("yaml")
	conf.viper.AddConfigPath(conf.viper.GetString("SERVICE_CONFIG_PATH"))
	err = conf.viper.ReadInConfig()
	notFound := false
	_, notFound = err.(viper.ConfigFileNotFoundError)
	if err != nil && !notFound {
		logger.Fatal("failed to read seascape.yml", "error", err)
	} else if notFound {
		logger.Warn("the seascape.yml configuration wasn't found", "engine error", err)
		return &conf, nil
	} else {
		conf.unmarshalService()
	}

	return &conf, nil
}

// unmarshalService decodes the yaml into the configuration.
func (config *Config) unmarshalService() {
	services, ok := config.viper.Get("services").([]interface{})
	if !ok {
		config.logger.Info("services", "Service", services, "raw", config.viper.Get("services"))
		config.logger.Fatal("seascape.yml Service should be a list not a one object")
	}

	if len(services) == 0 {
		config.logger.Warn("missing services in the configuration")
		return
	}

	kv, err := key_value.NewFromInterface(services[0])
	if err != nil {
		config.logger.Fatal("failed to convert raw config service into map", "error", err)
	}
	err = kv.Interface(config.Service)
	if err != nil {
		config.logger.Fatal("failed to convert raw config service to configuration.Service", "error", err)
	}
	err = config.Service.ValidateTypes()
	if err != nil {
		config.logger.Fatal("configuration.Service.ValidateTypes", "error", err)
	}
	err = config.Service.Lint()
	if err != nil {
		config.logger.Fatal("configuration.Service.Lint", "error", err)
	}
	config.logger.Info("todo", "todo 1", "make sure that proxy pipeline is correct",
		"todo 2", "make sure that only one kind of proxies are given",
		"todo 3", "make sure that only one kind of extensions are given",
		"todo 4", "make sure that services are all of the same kind but of different instance",
		"todo 5", "make sure that all controllers have the unique name in the config")
}

// Engine returns the underlying configuration engine.
// In our case it will be Viper.
func (config *Config) Engine() *viper.Viper {
	return config.viper
}

// SetDefaults sets the default configuration parameters.
func (config *Config) SetDefaults(defaultConfig DefaultConfig) {
	for name, value := range defaultConfig.Parameters {
		if value == nil {
			continue
		}
		// already set, don't use the default
		if config.viper.IsSet(name) {
			continue
		}
		config.logger.Info("Set default for "+defaultConfig.Title, name, value)
		config.SetDefault(name, value)
	}
}

// SetDefault sets the default configuration name to the value
func (config *Config) SetDefault(name string, value interface{}) {
	config.viper.SetDefault(name, value)
}

// Exist Checks whether the configuration variable exists or not
// If the configuration exists or its default value exists, then returns true.
func (config *Config) Exist(name string) bool {
	value := config.viper.GetString(name)
	return len(value) > 0
}

// GetString Returns the configuration parameter as a string
func (config *Config) GetString(name string) string {
	value := config.viper.GetString(name)
	return value
}

// GetUint64 Returns the configuration parameter as an unsigned 64-bit number
func (config *Config) GetUint64(name string) uint64 {
	value := config.viper.GetUint64(name)
	return value
}

// GetBool Returns the configuration parameter as a boolean
func (config *Config) GetBool(name string) bool {
	value := config.viper.GetBool(name)
	return value
}
