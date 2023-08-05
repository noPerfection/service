// Package configuration defines a configuration engine for the entire app.
//
// The configuration features:
//   - reads the command line arguments for the app such as authentication enabled or not.
//   - automatically loads the environment variables files.
//   - Allows setting default variables if user didn't define them.
package configuration

import (
	"fmt"
	"github.com/ahmetson/service-lib/configuration/argument"
	"github.com/ahmetson/service-lib/configuration/context"
	"github.com/ahmetson/service-lib/configuration/context/dev"
	"github.com/ahmetson/service-lib/configuration/path"
	"github.com/ahmetson/service-lib/configuration/service"
	"github.com/fsnotify/fsnotify"
	"path/filepath"
	"time"

	"github.com/ahmetson/service-lib/configuration/env"
	"github.com/ahmetson/service-lib/log"
	"github.com/spf13/viper"
)

// Config Configuration Engine based on viper.Viper
type Config struct {
	Name  string       // application name
	viper *viper.Viper // used to keep default values

	// Passed as --secure command line argument.
	// If it's passed, then authentication is switched off.
	Secure       bool
	logger       *log.Logger // debug purpose only
	Service      *service.Service
	Context      *context.Context
	handleChange func(*service.Service, error)
}

// New creates a global configuration for the entire application.
//
// Automatically reads the command line arguments.
// Loads the environment variables.
//
// Logger should be a parent
func New(parent *log.Logger) (*Config, error) {
	config := Config{
		Name:         parent.Prefix(),
		logger:       parent.Child("configuration"),
		Service:      nil,
		handleChange: nil,
	}
	config.logger.Info("Loading environment files passed as app arguments")

	// First, we load the environment variables
	err := env.LoadAnyEnv()
	if err != nil {
		return nil, fmt.Errorf("loading environment variables: %w", err)
	}

	paths, _ := argument.GetEnvPaths()
	config.logger.Info("Starting Viper with environment variables", "loaded files", paths)

	// replace the values with the ones we fetched from environment variables
	config.viper = viper.New()
	config.viper.AutomaticEnv()

	execPath, err := path.GetExecPath()
	if err != nil {
		return nil, fmt.Errorf("path.GetExecPath: %w", err)
	}

	// Use the service configuration given from the path
	if argument.Exist(argument.Configuration) {
		configurationPath, err := argument.Value(argument.Configuration)
		if err != nil {
			return nil, fmt.Errorf("failed to get the configuration path: %w", err)
		}

		absPath := path.GetPath(execPath, configurationPath)

		dir, fileName := path.SplitServicePath(absPath)
		config.viper.Set("SERVICE_CONFIG_NAME", fileName)
		config.viper.Set("SERVICE_CONFIG_PATH", dir)
	} else {
		config.viper.SetDefault("SERVICE_CONFIG_NAME", "service")
		config.viper.SetDefault("SERVICE_CONFIG_PATH", execPath)
	}

	contextDefault, err := dev.GetDefaultConfigs()
	if err != nil {
		return nil, fmt.Errorf("context.GetDefaultConfigs: %w", err)
	}
	config.SetDefaults(*contextDefault)
	devContext, err := dev.New(&config)
	if err != nil {
		return nil, fmt.Errorf("context.NewDev: %w", err)
	}
	config.Context = devContext

	configName := config.viper.GetString("SERVICE_CONFIG_NAME")
	configPath := config.viper.GetString("SERVICE_CONFIG_PATH")
	// load the service configuration
	config.viper.SetConfigName(configName)
	config.viper.SetConfigType("yaml")
	config.viper.AddConfigPath(configPath)

	serviceConfig, err := config.readFile()
	if err != nil {
		config.logger.Fatal("config.readFile", "error", err)
	} else {
		config.Service = serviceConfig
	}

	return &config, nil
}

// readFile reads the yaml into the interface{} in the engine, then
// it will unmarshall it into the config.Service.
//
// If the file doesn't exist, it will skip it without changing the old service
func (config *Config) readFile() (*service.Service, error) {
	err := config.viper.ReadInConfig()
	notFound := false
	_, notFound = err.(viper.ConfigFileNotFoundError)
	if err != nil && !notFound {
		return nil, fmt.Errorf("read '%s' failed: %w", config.viper.GetString("SERVICE_CONFIG_NAME"), err)
	} else if notFound {
		config.logger.Warn("yaml in path not found", "config", config.GetServicePath(), "engine error", err)
		return nil, nil
	}
	config.logger.Info("yaml was loaded, let's parse it")
	services, ok := config.viper.Get("services").([]interface{})
	if !ok {
		config.logger.Info("services", "Service", services, "raw", config.viper.Get("services"))
		return nil, fmt.Errorf("configuration.yml Service should be a list not a one object")
	}

	config.logger.Info("todo", "todo 1", "make sure that proxy pipeline is correct",
		"todo 2", "make sure that only one kind of proxies are given",
		"todo 3", "make sure that only one kind of extensions are given",
		"todo 4", "make sure that services are all of the same kind but of different instance",
		"todo 5", "make sure that all controllers have the unique name in the config")

	serviceConfig, err := service.UnmarshalService(services)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling service configuration failed: %w", err)
	}

	return serviceConfig, nil
}

func (config *Config) GetServicePath() string {
	configName := config.viper.GetString("SERVICE_CONFIG_NAME")
	configPath := config.viper.GetString("SERVICE_CONFIG_PATH")

	return filepath.Join(configPath, configName+".yml")
}

// Engine returns the underlying configuration engine.
// In our case, it will be Viper.
func (config *Config) Engine() *viper.Viper {
	return config.viper
}

// Watch tracks the configuration change in the file.
//
// Watch could be called only once. If it's already called, then it will skip it without an error.
//
// For production, we could call config.viper.WatchRemoteConfig() for example in etcd.
func (config *Config) Watch(watchHandle func(*service.Service, error)) error {
	if config.handleChange != nil {
		return nil
	}

	servicePath := config.GetServicePath()

	exists, err := path.FileExists(servicePath)
	if err != nil {
		return fmt.Errorf("FileExists('%s'): %w", servicePath, err)
	}

	// set it after checking for errors
	config.handleChange = watchHandle

	if !exists {
		// wait file appearance, then call the watchChange
		go config.watchFileCreation()
	} else {
		config.watchChange()
	}

	return nil
}

// If the file not exists, then watch for its appearance.
func (config *Config) watchFileCreation() {
	servicePath := config.GetServicePath()
	for {
		exists, err := path.FileExists(servicePath)
		if err != nil {
			config.handleChange(nil, fmt.Errorf("watchFileCreation: FileExists: %w", err))
			break
		}
		if exists {
			serviceConfig, err := config.readFile()
			if err != nil {
				config.handleChange(nil, fmt.Errorf("watchFileCreation: config.readFile: %w", err))
				break
			}

			config.handleChange(serviceConfig, nil)

			config.watchChange()
			break
		}
		time.Sleep(time.Millisecond * 200)
	}
}

// If file exists, then watch file deletion.
func (config *Config) watchFileDeletion() {
	servicePath := config.GetServicePath()
	for {
		exists, err := path.FileExists(servicePath)
		if err != nil {
			config.handleChange(nil, fmt.Errorf("watchFileDeletion: FileExists: %w", err))
			break
		}
		if !exists {
			config.handleChange(nil, nil)

			go config.watchFileCreation()
			break
		}
		time.Sleep(time.Millisecond * 200)
	}
}

func (config *Config) watchChange() {
	go config.watchFileDeletion()
	// if file not exists, call the file appearance

	config.logger.Warn("calling watch config")
	config.viper.WatchConfig()
	config.viper.OnConfigChange(func(e fsnotify.Event) {
		newConfig, err := config.readFile()
		if err != nil {
			config.handleChange(nil, fmt.Errorf("watchChange: config.readFile: %w", err))
		} else {
			config.handleChange(newConfig, nil)
		}
	})
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
