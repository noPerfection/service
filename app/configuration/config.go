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
func NewAppConfig(logger log.Logger) (*Config, error) {
	config_logger, err := logger.Child("app-config", log.WITHOUT_TIMESTAMP)
	if err != nil {
		return nil, fmt.Errorf("error creating child logger: %w", err)
	}

	// First we check the parameters of the application arguments
	arguments := argument.GetArguments(&config_logger)

	conf := Config{
		Secure:        argument.Has(arguments, argument.SECURE),
		DebugSecurity: argument.Has(arguments, argument.SECURITY_DEBUG),
		logger:        &config_logger,
	}

	config_logger.Info("Supported application arguments:")
	config_logger.Info("--"+argument.SECURE, "to enable authentication of TCP sockets. Enabled", conf.Secure)
	config_logger.Info("--"+argument.SECURITY_DEBUG, "to hide security debug. Enabled", conf.DebugSecurity)

	// First we load the environment variables
	err = env.LoadAnyEnv()
	if err != nil {
		return nil, fmt.Errorf("loading environment variables: %w", err)
	}

	// replace the values with the ones we fetched from environment variables
	conf.viper = viper.New()
	conf.viper.AutomaticEnv()

	return &conf, nil
}

// Set the default configuration parameters.
func (config *Config) SetDefaults(default_config DefaultConfig) {
	if config.logger != nil {
		config.logger.Info("Set the default config parameters for", "title", default_config.Title)
	}

	for name, value := range default_config.Parameters {
		if value == nil {
			continue
		}
		if config.logger != nil {
			config.logger.Info("default", name, value)
		}
		config.SetDefault(name, value)
	}
}

// Sets the default configuration name to the value
func (c *Config) SetDefault(name string, value interface{}) {
	// log.Printf("\tdefault config %s=%v", name, value)
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
