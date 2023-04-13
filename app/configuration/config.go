// Package configuration defines a configuration engine for the entire app.
//
// The configuration features:
//   - reads the command line arguments for the app such as authentication enabled or not.
//   - automatically loads the environment variables files.
//   - allows setting default variables if user didn't define them.
package configuration

import (
	"fmt"

	"github.com/blocklords/sds/app/configuration/argument"
	"github.com/blocklords/sds/app/configuration/env"
	"github.com/blocklords/sds/app/log"
	"github.com/spf13/viper"
)

// Configuration Engine based on viper.Viper
type Config struct {
	viper *viper.Viper // used to keep default values

	Secure        bool        // Passed as --secure command line argument. If its passed then authentication is switched off.
	DebugSecurity bool        // Passed as --debug-security command line argument. If true then app prints the security logs.
	logger        *log.Logger // debug purpose only
}

// NewAppConfig creates a global configuration for the entire application.
// Automatically reads the command line arguments.
// Loads the environment variables.
func NewAppConfig(parent log.Logger) (*Config, error) {
	logger, err := parent.Child("configuration", log.WITH_TIMESTAMP)
	if err != nil {
		return nil, fmt.Errorf("error creating child logger: %w", err)
	}
	logger.Info("Reading command line arguments for application parameters")

	// First we check the parameters of the application arguments
	arguments := argument.GetArguments(&logger)

	conf := Config{
		Secure:        argument.Has(arguments, argument.SECURE),
		DebugSecurity: argument.Has(arguments, argument.SECURITY_DEBUG),
		logger:        &logger,
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

	return &conf, nil
}

// Set the default configuration parameters.
func (config *Config) SetDefaults(default_config DefaultConfig) {
	config.logger.Info("Set the default config parameters for", "title", default_config.Title)

	for name, value := range default_config.Parameters {
		if value == nil {
			continue
		}
		config.logger.Info(default_config.Title, name, value)
		config.SetDefault(name, value)
	}
}

// Sets the default configuration name to the value
func (c *Config) SetDefault(name string, value interface{}) {
	c.viper.SetDefault(name, value)
}

// Checks whether the configuration variable exists or not
// If the configuration exists or its default value exists, then returns true.
func (c *Config) Exist(name string) bool {
	value := c.viper.GetString(name)
	return len(value) > 0
}

// Returns the configuration parameter as a string
func (c *Config) GetString(name string) string {
	value := c.viper.GetString(name)
	return value
}

// Returns the configuration parameter as an unsigned 64 bit number
func (c *Config) GetUint64(name string) uint64 {
	value := c.viper.GetUint64(name)
	return value
}

// Returns the configuration parameter as a boolean
func (c *Config) GetBool(name string) bool {
	value := c.viper.GetBool(name)
	return value
}
