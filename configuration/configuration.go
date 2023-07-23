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
	"github.com/phayes/freeport"
	"gopkg.in/yaml.v3"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/ahmetson/service-lib/configuration/env"
	"github.com/ahmetson/service-lib/log"
	"github.com/spf13/viper"
)

// Config Configuration Engine based on viper.Viper
type Config struct {
	Name  string       // application name
	viper *viper.Viper // used to keep default values

	Secure  bool        // Passed as --secure command line argument. If its passed then authentication is switched off.
	logger  *log.Logger // debug purpose only
	Service Service
	Context *Context
}

// GetCurrentPath returns the current path of the executable
func GetCurrentPath() (string, error) {
	ex, err := os.Executable()
	if err != nil {
		return "", err
	}
	exPath := filepath.Dir(ex)
	return exPath, nil
}

// New creates a global configuration for the entire application.
//
// Automatically reads the command line arguments.
// Loads the environment variables.
//
// logger should be a parent
func New(parent *log.Logger) (*Config, error) {
	config := Config{
		Name:    parent.Prefix(),
		logger:  parent.Child("configuration"),
		Service: Service{},
	}
	parent.Info("Loading environment files passed as app arguments")

	// First we load the environment variables
	err := env.LoadAnyEnv()
	if err != nil {
		return nil, fmt.Errorf("loading environment variables: %w", err)
	}

	parent.Info("Starting Viper with environment variables")

	// replace the values with the ones we fetched from environment variables
	config.viper = viper.New()
	config.viper.AutomaticEnv()

	// Use the service configuration given from the path
	if argument.Exist(argument.Configuration) {
		configurationPath, err := argument.Value(argument.Configuration)
		if err != nil {
			return nil, fmt.Errorf("failed to get the configuration path: %w", err)
		}

		if err := validateServicePath(configurationPath); err != nil {
			return nil, fmt.Errorf("configuration path '%s' validation: %w", configurationPath, err)
		}
		dir, fileName := splitServicePath(configurationPath)
		config.viper.Set("SERVICE_CONFIG_NAME", fileName)
		config.viper.Set("SERVICE_CONFIG_PATH", dir)
	} else {
		config.viper.SetDefault("SERVICE_CONFIG_NAME", "service")
		config.viper.SetDefault("SERVICE_CONFIG_PATH", ".")
	}

	// set up the context
	parent.Info("the context before", config.Context)
	initContext(&config)
	setDevContext(&config)
	parent.Info("the context after", config.Context)

	// load the service configuration
	config.viper.SetConfigName(config.viper.GetString("SERVICE_CONFIG_NAME"))
	config.viper.SetConfigType("yaml")
	config.viper.AddConfigPath(config.viper.GetString("SERVICE_CONFIG_PATH"))

	err = config.viper.ReadInConfig()
	notFound := false
	_, notFound = err.(viper.ConfigFileNotFoundError)
	if err != nil && !notFound {
		return nil, fmt.Errorf("failed to read configuration %s: %w", config.viper.GetString("SERVICE_CONFIG_NAME"), err)
	} else if notFound {
		parent.Warn("the configuration.yml configuration wasn't found", "engine error", err)
		return &config, nil
	} else {
		services, ok := config.viper.Get("services").([]interface{})
		if !ok {
			config.logger.Info("services", "Service", services, "raw", config.viper.Get("services"))
			config.logger.Fatal("configuration.yml Service should be a list not a one object")
		}

		config.logger.Info("todo", "todo 1", "make sure that proxy pipeline is correct",
			"todo 2", "make sure that only one kind of proxies are given",
			"todo 3", "make sure that only one kind of extensions are given",
			"todo 4", "make sure that services are all of the same kind but of different instance",
			"todo 5", "make sure that all controllers have the unique name in the config")

		service, err := UnmarshalService(services)
		if err != nil {
			config.logger.Fatal("unmarshalling service configuration failed", "error", err)
		}
		config.Service = service
	}

	return &config, nil
}

// GetFreePort returns a TCP port to use
func (config *Config) GetFreePort() int {
	port, err := freeport.GetFreePort()
	if err != nil {
		config.logger.Fatal("kernel error", "error", err)
	}

	return port
}

// UnmarshalService decodes the yaml into the configuration.
func UnmarshalService(services []interface{}) (Service, error) {
	if len(services) == 0 {
		return Service{}, nil
	}

	kv, err := key_value.NewFromInterface(services[0])
	if err != nil {
		return Service{}, fmt.Errorf("failed to convert raw config service into map: %w", err)
	}

	var service Service
	err = kv.Interface(&service)
	if err != nil {
		return Service{}, fmt.Errorf("failed to convert raw config service to configuration.Service: %w", err)
	}
	err = prepareService(&service)
	if err != nil {
		return Service{}, fmt.Errorf("prepareService: %w", err)
	}

	return service, nil
}

func prepareService(service *Service) error {
	err := service.ValidateTypes()
	if err != nil {
		return fmt.Errorf("service.ValidateTypes: %w", err)
	}
	err = service.Lint()
	if err != nil {
		return fmt.Errorf("service.Lint: %w", err)
	}

	return nil
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

// validateServicePath returns an error if the path is not a valid .yml link
func validateServicePath(path string) error {
	if len(path) < 5 {
		return fmt.Errorf("path is too short")
	}
	_, found := strings.CutSuffix(path, ".yml")
	if !found {
		return fmt.Errorf("the path should end with '.yml'")
	}

	return nil
}

// splitServicePath returns the directory, file name from the given path.
// the extension is not returned since it's always a yaml file.
//
// The function doesn't validate the path.
// Therefore, call this function after validateServicePath()
func splitServicePath(servicePath string) (string, string) {
	dir, fileName := path.Split(servicePath)

	if len(dir) == 0 {
		dir = "."
	}

	fileName = fileName[0 : len(fileName)-4]

	return dir, fileName
}

func ReadService(path string) (Service, error) {
	if err := validateServicePath(path); err != nil {
		return Service{}, fmt.Errorf("validateServicePath: %w", err)
	}

	bytes, err := os.ReadFile(path)
	if err != nil {
		return Service{}, fmt.Errorf("os.ReadFile of %s: %w", path, err)
	}

	yamlConfig := createYaml()
	kv := yamlConfig.Map()
	err = yaml.Unmarshal(bytes, &kv)

	if err != nil {
		return Service{}, fmt.Errorf("yaml.Unmarshal of %s: %w", path, err)
	}

	fmt.Println("service", kv)

	yamlConfig = key_value.New(kv)
	if err := yamlConfig.Exist("Services"); err != nil {
		return Service{}, fmt.Errorf("no services in yaml: %w", err)
	}

	services, err := yamlConfig.GetKeyValueList("Services")
	if err != nil {
		return Service{}, fmt.Errorf("failed to get services as key value list: %w", err)
	}

	if len(services) == 0 {
		return Service{}, fmt.Errorf("no services in the configuration")
	}

	var service Service
	err = services[0].Interface(&service)
	if err != nil {
		return Service{}, fmt.Errorf("convert key value to Service: %w", err)
	}

	err = prepareService(&service)
	if err != nil {
		return Service{}, fmt.Errorf("prepareService: %w", err)
	}

	return service, nil
}

func createYaml(configs ...Service) key_value.KeyValue {
	var services = configs
	kv := key_value.Empty()
	kv.Set("Services", services)

	return kv
}

// WriteService writes the service as the yaml on the given path.
// If the path doesn't contain the file extension it will through an error
func WriteService(path string, service Service) error {
	if err := validateServicePath(path); err != nil {
		return fmt.Errorf("validateServicePath: %w", err)
	}

	kv := createYaml(service)

	serviceConfig, err := yaml.Marshal(kv.Map())
	if err != nil {
		return fmt.Errorf("failed to marshall config.Service: %w", err)
	}

	f, _ := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	_, err = f.Write(serviceConfig)
	closeErr := f.Close()
	if err != nil {
		return fmt.Errorf("failed to write service into the given path: %w", err)
	} else if closeErr != nil {
		return fmt.Errorf("failed to close the file descriptor: %w", closeErr)
	} else {
		return nil
	}
}
