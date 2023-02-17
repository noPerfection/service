package configuration

import (
	"fmt"

	"github.com/blocklords/gosds/app/argument"
	"github.com/blocklords/gosds/app/env"
	"github.com/blocklords/gosds/common/data_type/key_value"
	"github.com/spf13/viper"
)

// Application configuration
type Config struct {
	viper *viper.Viper

	Plain     bool // if true then no security
	Broadcast bool // if true then broadcast of the service will be enabled
	Reply     bool // if true then reply controller of the service will be enabled
}

// Returns the new configuration file after loading environment variables
func New() (*Config, error) {
	// First we check the parameters of the application arguments
	arguments, err := argument.GetArguments()
	if err != nil {
		return nil, fmt.Errorf("reading application arguments: %v", err)
	}

	conf := Config{
		Plain:     argument.Has(arguments, argument.PLAIN),
		Broadcast: argument.Has(arguments, argument.BROADCAST),
		Reply:     argument.Has(arguments, argument.REPLY),
	}

	// First we load the environment variables
	err = env.LoadAnyEnv()
	if err != nil {
		return nil, fmt.Errorf("loading environment variables: %v", err)
	}

	// replace the values with the ones we fetched from environment variables
	conf.viper = viper.New()
	conf.viper.AutomaticEnv()

	return &conf, nil
}

// Populates the app configuration with the default vault configuration parameters.
func (config *Config) SetDefaults(kv key_value.KeyValue) {
	for name, value := range kv {
		if value == nil {
			continue
		}

		config.SetDefault(name, value)
	}
}

// Sets the default value
func (c *Config) SetDefault(name string, value interface{}) {
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
