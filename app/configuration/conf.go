package configuration

import (
	"fmt"
	"log"

	"github.com/blocklords/gosds/app/argument"
	"github.com/blocklords/gosds/app/env"
	"github.com/spf13/viper"
)

// Application configuration
type Config struct {
	viper *viper.Viper

	Plain         bool // if true then no security
	Broadcast     bool // if true then broadcast of the service will be enabled
	Reply         bool // if true then reply controller of the service will be enabled
	DebugSecurity bool // if true then we print the security layer logs
}

// Returns the new configuration file after loading environment variables
// At the application level
func NewAppConfig() (*Config, error) {
	// First we check the parameters of the application arguments
	arguments, err := argument.GetArguments()
	if err != nil {
		return nil, fmt.Errorf("reading application arguments: %v", err)
	}

	conf := Config{
		Plain:         argument.Has(arguments, argument.PLAIN),
		Broadcast:     argument.Has(arguments, argument.BROADCAST),
		Reply:         argument.Has(arguments, argument.REPLY),
		DebugSecurity: argument.Has(arguments, argument.SECURITY_DEBUG),
	}

	log.Println("Supported application arguments:")
	log.Printf("--%s to switch off security. enabled: %v\n", argument.PLAIN, conf.Plain)
	log.Printf("--%s to enable broadcast. enabled: %v\n", argument.BROADCAST, conf.Broadcast)
	log.Printf("--%s to enable controller. enabled: %v\n\n", argument.REPLY, conf.Reply)

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

// Return the configuration engine to use with default parameters
func New() (*Config, error) {
	// First we check the parameters of the application arguments
	arguments, err := argument.GetArguments()
	if err != nil {
		return nil, fmt.Errorf("reading application arguments: %v", err)
	}

	conf := Config{
		Plain:     argument.Has(arguments, argument.PLAIN),
		Broadcast: false,
		Reply:     false,
	}

	// replace the values with the ones we fetched from environment variables
	conf.viper = viper.New()
	conf.viper.AutomaticEnv()

	return &conf, nil
}

// Returns the configuration for the service
// That means application arguments are not used.
// Only the underlying configuration engine is loaded.
func NewService(default_config DefaultConfig) (*Config, error) {
	// First we check the parameters of the application arguments
	arguments, err := argument.GetArguments()
	if err != nil {
		return nil, fmt.Errorf("reading application arguments: %v", err)
	}

	conf := Config{
		Plain:     argument.Has(arguments, argument.PLAIN),
		Broadcast: false,
		Reply:     false,
	}

	// replace the values with the ones we fetched from environment variables
	conf.viper = viper.New()
	conf.viper.AutomaticEnv()

	conf.SetDefaults(default_config)

	return &conf, nil
}

// Populates the app configuration with the default vault configuration parameters.
func (config *Config) SetDefaults(default_config DefaultConfig) {
	log.Printf("'%s' default values. Override on environment variables\n", default_config.Title)

	for name, value := range default_config.Parameters {
		if value == nil {
			continue
		}
		config.SetDefault(name, value)
	}

	log.Printf("\n\n")
}

// Sets the default value
func (c *Config) SetDefault(name string, value interface{}) {
	log.Printf("\tdefault config %s=%v", name, value)
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
