// The app configuration package is the wrapper around Viper.
// It's the way to work with environment variables.
// It also provides the default parameters.
package configuration

import (
	"fmt"

	"github.com/charmbracelet/log"

	"github.com/blocklords/sds/app/configuration/argument"
	"github.com/blocklords/sds/app/configuration/env"
	app_log "github.com/blocklords/sds/app/log"
	"github.com/spf13/viper"
)

// Configuration Engine
type Config struct {
	viper *viper.Viper // used to keep default values

	Plain         bool       // if true then no security
	Broadcast     bool       // if true then broadcast of the service will be enabled
	Reply         bool       // if true then reply controller of the service will be enabled
	DebugSecurity bool       // if true then we print the security layer logs
	logger        log.Logger // debug purpose only
}

// Returns the new configuration file after loading environment variables
// At the application level
func NewAppConfig(logger log.Logger) (*Config, error) {
	config_logger := app_log.Child(logger, "app-config")
	config_logger.SetReportCaller(false)
	config_logger.SetReportTimestamp(false)
	// First we check the parameters of the application arguments
	arguments := argument.GetArguments(config_logger)

	conf := Config{
		Plain:         argument.Has(arguments, argument.PLAIN),
		DebugSecurity: argument.Has(arguments, argument.SECURITY_DEBUG),
		logger:        config_logger,
	}

	config_logger.Info("Supported application arguments:")
	config_logger.Info("--"+argument.PLAIN, "to switch off security. Enabled", conf.Plain)
	config_logger.Info("--"+argument.SECURITY_DEBUG, "to hide security debug. Enabled", conf.DebugSecurity)

	// First we load the environment variables
	err := env.LoadAnyEnv()
	if err != nil {
		return nil, fmt.Errorf("loading environment variables: %w", err)
	}

	// replace the values with the ones we fetched from environment variables
	conf.viper = viper.New()
	conf.viper.AutomaticEnv()

	return &conf, nil
}

// Return the configuration engine to use with default parameters
func New() *Config {
	arguments := argument.GetArguments(nil)

	conf := Config{
		Plain:     argument.Has(arguments, argument.PLAIN),
		Broadcast: false,
		Reply:     false,
	}

	// replace the values with the ones we fetched from environment variables
	conf.viper = viper.New()
	conf.viper.AutomaticEnv()

	return &conf
}

// Returns the configuration for the service
// That means application arguments are not used.
// Only the underlying configuration engine is loaded.
func NewService(default_config DefaultConfig) *Config {
	// First we check the parameters of the application arguments
	arguments := argument.GetArguments(nil)

	conf := Config{
		Plain:     argument.Has(arguments, argument.PLAIN),
		Broadcast: false,
		Reply:     false,
	}

	// replace the values with the ones we fetched from environment variables
	conf.viper = viper.New()
	conf.viper.AutomaticEnv()

	conf.SetDefaults(default_config)

	return &conf
}

// Populates the app configuration with the default vault configuration parameters.
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

// Sets the default value
func (c *Config) SetDefault(name string, value interface{}) {
	// log.Printf("\tdefault config %s=%v", name, value)
	c.viper.SetDefault(name, value)
}

// Checks whether the configuration variable exists or not
func (c *Config) Exist(name string) bool {
	value := c.viper.GetString(name)
	return len(value) > 0
}

// Returns the configuration parameter as a string
func (c *Config) GetString(name string) string {
	value := c.viper.GetString(name)
	return value
}

// Returns the configuration parameter as a number
func (c *Config) GetUint64(name string) uint64 {
	value := c.viper.GetUint64(name)
	return value
}

// Returns the configuration parameter as a boolean
func (c *Config) GetBool(name string) bool {
	value := c.viper.GetBool(name)
	return value
}
