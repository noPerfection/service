package configuration

import (
	"github.com/ahmetson/service-lib/configuration/path"
)

type ContextType = string

const SrcKey = "SERVICE_DEPS_SRC"
const BinKey = "SERVICE_DEPS_BIN"
const DataKey = "SERVICE_DEPS_DATA"

// DevContext indicates that all dependency proxies are in the local machine
const DevContext ContextType = "development"

// DefaultContext indicates that all dependencies are in any machine.
// It's unspecified.
const DefaultContext ContextType = "default"

// A Context handles the configuration of the contexts
type Context struct {
	Type ContextType `json:"CONTEXT_TYPE"`
	Src  string      `json:"SERVICE_DEPS_SRC"`
	Bin  string      `json:"SERVICE_DEPS_BIN"`
	Data string      `json:"SERVICE_DEPS_DATA"`
	url  string
}

func initContext(config *Config) {
	exePath, err := path.GetExecPath()
	if err != nil {
		config.logger.Fatal("failed to get the executable path", "error", err)
	}

	config.viper.SetDefault(SrcKey, path.GetPath(exePath, "./deps/.src"))
	config.viper.SetDefault(BinKey, path.GetPath(exePath, "./deps/.bin"))
	config.viper.SetDefault(DataKey, path.GetPath(exePath, "./deps/.data"))
}

func newContext(config *Config) *Context {
	execPath, err := path.GetExecPath()
	if err != nil {
		config.logger.Fatal("path.GetExecPath: %w", err)
	}
	srcPath := path.GetPath(execPath, config.viper.GetString(SrcKey))
	dataPath := path.GetPath(execPath, config.viper.GetString(DataKey))
	binPath := path.GetPath(execPath, config.viper.GetString(BinKey))

	config.logger.Info("context paths", "source", srcPath, "data", dataPath, "bin", binPath)

	return &Context{
		Src:  srcPath,
		Bin:  binPath,
		Data: dataPath,
	}
}

func setDevContext(config *Config) {
	context := newContext(config)
	context.Type = DevContext

	config.Context = context
}

func (context *Context) SetUrl(url string) {
	context.url = url
}

func (context *Context) GetUrl() string {
	return context.url
}

func (context *Context) Host() string {
	if context.Type == DevContext {
		return "localhost"
	}
	return "0.0.0.0"
}
