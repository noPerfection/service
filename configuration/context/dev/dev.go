// Package dev in the configuration package handles the dev context data.
// In the dev context, the configuration files are stored in the local filesystem.
package dev

import (
	"fmt"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/service-lib/configuration"
	"github.com/ahmetson/service-lib/configuration/context"
	"github.com/ahmetson/service-lib/configuration/path"
	"github.com/ahmetson/service-lib/configuration/service"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strings"
)

const (
	SrcKey  = "SERVICE_DEPS_SRC"
	BinKey  = "SERVICE_DEPS_BIN"
	DataKey = "SERVICE_DEPS_DATA"
)

// A Context handles the configuration of the contexts
type Context struct {
	Src  string `json:"SERVICE_DEPS_SRC"`
	Bin  string `json:"SERVICE_DEPS_BIN"`
	Data string `json:"SERVICE_DEPS_DATA"`
	url  string
}

func GetDefaultConfigs() (*configuration.DefaultConfig, error) {
	exePath, err := path.GetExecPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get the executable path: %w", err)
	}

	return &configuration.DefaultConfig{
		Title: "Interface",
		Parameters: key_value.Empty().
			Set(SrcKey, path.GetPath(exePath, "./deps/.src")).
			Set(BinKey, path.GetPath(exePath, "./deps/.bin")).
			Set(DataKey, path.GetPath(exePath, "./deps/.data")),
	}, nil
}

// New creates a dev context
func New(config *configuration.Config) (*Context, error) {
	execPath, err := path.GetExecPath()
	if err != nil {
		return nil, fmt.Errorf("path.GetExecPath: %w", err)
	}
	srcPath := path.GetPath(execPath, config.Engine().GetString(SrcKey))
	dataPath := path.GetPath(execPath, config.Engine().GetString(DataKey))
	binPath := path.GetPath(execPath, config.Engine().GetString(BinKey))

	devContext := &Context{
		Src:  srcPath,
		Bin:  binPath,
		Data: dataPath,
	}

	if err != nil {
		return nil, fmt.Errorf("newContext: %w", err)
	}
	return devContext, nil
}

func (c *Context) GetType() context.Type {
	return context.DevContext
}

// ReadService on the given path.
// If a path is not obsolete, then it should be relative to the executable.
// The path should have the .yml extension
func (c *Context) ReadService(path string) (*service.Service, error) {
	if err := validateServicePath(path); err != nil {
		return nil, fmt.Errorf("validateServicePath: %w", err)
	}

	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("os.ReadFile of %s: %w", path, err)
	}

	yamlConfig := createYaml()
	kv := yamlConfig.Map()
	err = yaml.Unmarshal(bytes, &kv)

	if err != nil {
		return nil, fmt.Errorf("yaml.Unmarshal of %s: %w", path, err)
	}

	fmt.Println("service", kv)

	yamlConfig = key_value.New(kv)
	if err := yamlConfig.Exist("Services"); err != nil {
		return nil, fmt.Errorf("no services in yaml: %w", err)
	}

	services, err := yamlConfig.GetKeyValueList("Services")
	if err != nil {
		return nil, fmt.Errorf("failed to get services as key value list: %w", err)
	}

	if len(services) == 0 {
		return nil, fmt.Errorf("no services in the configuration")
	}

	var serviceConfig service.Service
	err = services[0].Interface(&serviceConfig)
	if err != nil {
		return nil, fmt.Errorf("convert key value to Service: %w", err)
	}

	err = serviceConfig.PrepareService()
	if err != nil {
		return nil, fmt.Errorf("prepareService: %w", err)
	}

	return &serviceConfig, nil
}

// WriteService writes the service as the yaml on the given path.
// If the path doesn't contain the file extension, it will through an error
func (c *Context) WriteService(path string, service *service.Service) error {
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

func createYaml(configs ...*service.Service) key_value.KeyValue {
	var services = configs
	kv := key_value.Empty()
	kv.Set("Services", services)

	return kv
}

func (c *Context) Paths() []string {
	return []string{c.Data, c.Bin, c.Src}
}

func (c *Context) SetUrl(url string) {
	c.url = url
}

func (c *Context) GetUrl() string {
	return c.url
}

func (c *Context) Host() string {
	return "localhost"
}

//----------------------------------------------------------
// below are the specific functions for the dev context. other contexts may not have them
//----------------------------------------------------------

// EnvPath is the shared configurations between dependencies
func (c *Context) EnvPath() string {
	return filepath.Join(c.Data, ".env")
}

// ConfigurationPath returns configuration url in the context's data
func (c *Context) ConfigurationPath(url string) string {
	fileName := configuration.UrlToFileName(c.GetUrl())
	return filepath.Join(c.Data, url, fileName+".yml")
}

func (c *Context) SrcPath(url string) string {
	return filepath.Join(c.Src, url)
}
