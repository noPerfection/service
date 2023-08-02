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
	"github.com/ahmetson/service-lib/configuration/path"
	"github.com/cakturk/go-netstat/netstat"
	"github.com/fsnotify/fsnotify"
	"github.com/phayes/freeport"
	"gopkg.in/yaml.v3"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ahmetson/service-lib/configuration/env"
	"github.com/ahmetson/service-lib/log"
	"github.com/spf13/viper"
)

// Config Configuration Engine based on viper.Viper
type Config struct {
	Name  string       // application name
	viper *viper.Viper // used to keep default values

	Secure   bool        // Passed as --secure command line argument. If its passed then authentication is switched off.
	logger   *log.Logger // debug purpose only
	Service  Service
	Context  *Context
	watching bool
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
	config.logger.Info("Loading environment files passed as app arguments")

	// First we load the environment variables
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

		if err := validateServicePath(absPath); err != nil {
			return nil, fmt.Errorf("configuration path '%s' validation: %w", absPath, err)
		}

		dir, fileName := path.SplitServicePath(absPath)
		config.viper.Set("SERVICE_CONFIG_NAME", fileName)
		config.viper.Set("SERVICE_CONFIG_PATH", dir)
	} else {
		config.viper.SetDefault("SERVICE_CONFIG_NAME", "service")
		config.viper.SetDefault("SERVICE_CONFIG_PATH", execPath)
	}

	// set up the context
	initContext(&config)
	setDevContext(&config)

	configName := config.viper.GetString("SERVICE_CONFIG_NAME")
	configPath := config.viper.GetString("SERVICE_CONFIG_PATH")
	// load the service configuration
	config.viper.SetConfigName(configName)
	config.viper.SetConfigType("yaml")
	config.viper.AddConfigPath(configPath)

	service, err := config.readFile()
	if err != nil {
		config.logger.Fatal("config.readFile", "error", err)
	} else {
		config.Service = service
	}

	return &config, nil
}

// readFile reads the yaml into the interface{} in the engine, then
// it will unmarshall it into the config.Service.
//
// If the file doesn't exist, it will skip it without changing the old service
func (config *Config) readFile() (Service, error) {
	err := config.viper.ReadInConfig()
	notFound := false
	_, notFound = err.(viper.ConfigFileNotFoundError)
	if err != nil && !notFound {
		return Service{}, fmt.Errorf("read '%s' failed: %w", config.viper.GetString("SERVICE_CONFIG_NAME"), err)
	} else if notFound {
		config.logger.Warn("yaml in path not found", "config", config.GetServicePath(), "engine error", err)
		return Service{}, nil
	}
	config.logger.Info("yaml was loaded, let's parse it")
	services, ok := config.viper.Get("services").([]interface{})
	if !ok {
		config.logger.Info("services", "Service", services, "raw", config.viper.Get("services"))
		return Service{}, fmt.Errorf("configuration.yml Service should be a list not a one object")
	}

	config.logger.Info("todo", "todo 1", "make sure that proxy pipeline is correct",
		"todo 2", "make sure that only one kind of proxies are given",
		"todo 3", "make sure that only one kind of extensions are given",
		"todo 4", "make sure that services are all of the same kind but of different instance",
		"todo 5", "make sure that all controllers have the unique name in the config")

	service, err := UnmarshalService(services)
	if err != nil {
		return Service{}, fmt.Errorf("unmarshalling service configuration failed: %w", err)
	}

	return service, nil
}

func (config *Config) GetServicePath() string {
	configName := config.viper.GetString("SERVICE_CONFIG_NAME")
	configPath := config.viper.GetString("SERVICE_CONFIG_PATH")

	return filepath.Join(configPath, configName+".yml")
}

// GetFreePort returns a TCP port to use
func (config *Config) GetFreePort() int {
	port, err := freeport.GetFreePort()
	if err != nil {
		config.logger.Fatal("kernel error", "error", err)
	}

	return port
}

func CurrentPid() uint64 {
	return uint64(os.Getpid())
}

func PortToPid[V int | uint64](port V) (uint64, error) {
	socks, err := netstat.TCPSocks(func(s *netstat.SockTabEntry) bool {
		return s.LocalAddr.Port == uint16(port)
	})
	if err != nil {
		return 0, fmt.Errorf("netstart.TCPSocks: %w", err)
	}
	if len(socks) == 0 {
		return 0, fmt.Errorf("no process on port %d: %w", port, err)
	}
	sock := socks[0]

	return uint64(sock.Process.Pid), nil
}

func IsPortUsed[V int | uint64](host string, port V) bool {
	portString := fmt.Sprintf("%d", port)
	timeout := time.Second
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, portString), timeout)
	if err != nil {
		return false
	}
	if conn != nil {
		conn.Close()
	}
	return true
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

// FileExists returns true if the file exists. if the path is a directory it will return false.
func FileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		} else {
			return false, fmt.Errorf("os.Stat('%s'): %w", path, err)
		}
	}

	if info.IsDir() {
		return false, fmt.Errorf("path('%s') is directory", path)
	}

	return true, nil
}

// Watch tracks the configuration change in the file.
//
// Watch could be called only once. If it's already called, then it will skip it without an error.
//
// For production, we could call config.viper.WatchRemoteConfig() for example in etcd.
func (config *Config) Watch() error {
	if config.watching {
		return nil
	}

	servicePath := config.GetServicePath()

	exists, err := FileExists(servicePath)
	if err != nil {
		return fmt.Errorf("FileExists('%s'): %w", servicePath, err)
	}

	// set it after checking for errors
	config.watching = true

	if !exists {
		// wait file appearance, then call the watchChange
		go config.watchFileCreation()
	} else {
		config.logger.Info("file exists, start engine watch")
		config.watchChange()
	}

	return nil
}

// If the file not exists, then watch for its appearance.
func (config *Config) watchFileCreation() {
	servicePath := config.GetServicePath()
	for {
		exists, err := FileExists(servicePath)
		if err != nil {
			config.logger.Error("FileExists", "service path", servicePath, "error", err)
		}
		if exists {
			config.logger.Info("file created, stop checking for updates and start engine watch. let's see what is in the file")
			service, err := config.readFile()
			if err != nil {
				config.logger.Fatal("reading file failed", "error", err)
			}

			config.logger.Info("comparing internal and changed configs", "changed", service, "internal", config.Service, "error to load changed", err)
			config.logger.Info("start tracking file changes call the callback for file change")

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
		exists, err := FileExists(servicePath)
		if err != nil {
			config.logger.Error("FileExists", "service path", servicePath, "error", err)
		}
		if !exists {
			config.logger.Info("file deleted, stop checking for updates and start engine watch. let's see what is in the file")
			service := Service{}
			config.logger.Info("comparing internal and changed configs", "changed", service, "internal", config.Service, "error to load changed", err)

			config.logger.Warn("file was moved, wait for it's creation")
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
		service, err := config.readFile()
		if err != nil {
			config.logger.Fatal("unmarshalling service configuration failed", "error", err)
		}

		config.logger.Info("comparing internal and changed configs", "changed", service, "internal", config.Service, "error to load changed", err)
		config.logger.Info("Config file changed", "name", e.Name, "op", e.Op.String(), "serialized", e.String())
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

// validateServicePath returns an error if the path is not a valid .yml link
func validateServicePath(path string) error {
	if len(path) < 5 || len(filepath.Base(path)) < 5 {
		return fmt.Errorf("path is too short")
	}
	_, found := strings.CutSuffix(path, ".yml")
	if !found {
		return fmt.Errorf("the path should end with '.yml'")
	}

	return nil
}

// ReadService on the given path.
// If path is not obsolete, then it should be relative to the executable.
// The path should have the .yml extension
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
