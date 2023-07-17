// Package configuration defines a configuration engine for the entire app.
//
// The configuration features:
//   - reads the command line arguments for the app such as authentication enabled or not.
//   - automatically loads the environment variables files.
//   - allows setting default variables if user didn't define them.
package configuration

import (
	"fmt"
	"github.com/Seascape-Foundation/sds-common-lib/data_type/key_value"

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
	Services      Services
}

// NewAppConfig creates a global configuration for the entire application.
// Automatically reads the command line arguments.
// Loads the environment variables.
func NewAppConfig(parent log.Logger) (*Config, error) {
	logger, err := parent.Child("configuration")
	if err != nil {
		return nil, fmt.Errorf("error creating child logger: %w", err)
	}
	logger.Info("Reading command line arguments for application parameters")

	// First we check the parameters of the application arguments
	arguments := argument.GetArguments(&logger)

	conf := Config{
		Secure:        argument.Has(arguments, argument.SECURE),
		DebugSecurity: argument.Has(arguments, argument.SecurityDebug),
		logger:        &logger,
		Services:      make(Services, 0),
	}
	logger.Info("Loading environment files passed as app arguments")

	// First we load the environment variables
	err = env.LoadAnyEnv()
	if err != nil {
		return nil, fmt.Errorf("loading environment variables: %w", err)
	}

	logger.Info("Starting Viper with environment variables")

	// replace the values with the ones we fetched from environment variables
	conf.viper = viper.New()
	conf.viper.AutomaticEnv()

	conf.viper.SetConfigName("seascape")
	conf.viper.SetConfigType("yaml")
	conf.viper.AddConfigPath(".")
	err = conf.viper.ReadInConfig()
	notFound := false
	_, notFound = err.(viper.ConfigFileNotFoundError)
	if err != nil && !notFound {
		logger.Fatal("failed to read seascape.yml", "error", err)
	} else if notFound {
		logger.Warn("the seascape.yml configuration wasn't found", "engine error", err)
		return &conf, nil
	}

	services, ok := conf.viper.Get("services").([]interface{})
	if !ok {
		logger.Info("services", "Services", services, "raw", conf.viper.Get("services"))
		logger.Fatal("seascape.yml Services should be a list not a one object")
	}

	for _, raw := range services {
		kv, err := key_value.NewFromInterface(raw)
		if err != nil {
			logger.Fatal("failed to convert raw config service into map", "error", err)
		}
		var serv Service
		err = kv.Interface(&serv)
		if err != nil {
			logger.Fatal("failed to convert raw config service to configuration.Service", "error", err)
		}
		err = serv.Validate()
		if err != nil {
			logger.Fatal("configuration.Service.Validate", "error", err)
		}
		err = serv.Lint()
		if err != nil {
			logger.Fatal("configuration.Service.Lint", "error", err)
		}
		logger.Info("todo", "todo 1", "make sure that proxy pipeline is correct",
			"todo 2", "make sure that only one kind of proxies are given",
			"todo 3", "make sure that only one kind of extensions are given",
			"todo 4", "make sure that services are all of the same kind but of different instance",
			"todo 5", "make sure that all controllers have the unique name in the config")
		conf.Services = append(conf.Services, serv)
	}

	return &conf, nil
}

// Engine returns the underlying configuration engine.
// In our case it will be Viper.
func (c *Config) Engine() *viper.Viper {
	return c.viper
}

// SetDefaults sets the default configuration parameters.
func (c *Config) SetDefaults(defaultConfig DefaultConfig) {
	for name, value := range defaultConfig.Parameters {
		if value == nil {
			continue
		}
		// already set, don't use the default
		if c.viper.IsSet(name) {
			continue
		}
		c.logger.Info("Set default for "+defaultConfig.Title, name, value)
		c.SetDefault(name, value)
	}
}

// SetDefault sets the default configuration name to the value
func (c *Config) SetDefault(name string, value interface{}) {
	c.viper.SetDefault(name, value)
}

// Exist Checks whether the configuration variable exists or not
// If the configuration exists or its default value exists, then returns true.
func (c *Config) Exist(name string) bool {
	value := c.viper.GetString(name)
	return len(value) > 0
}

// GetString Returns the configuration parameter as a string
func (c *Config) GetString(name string) string {
	value := c.viper.GetString(name)
	return value
}

// GetUint64 Returns the configuration parameter as an unsigned 64-bit number
func (c *Config) GetUint64(name string) uint64 {
	value := c.viper.GetUint64(name)
	return value
}

// GetBool Returns the configuration parameter as a boolean
func (c *Config) GetBool(name string) bool {
	value := c.viper.GetBool(name)
	return value
}
