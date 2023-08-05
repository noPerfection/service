package context

import (
	"fmt"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/service-lib/configuration"
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

func GetDefaultConfigs() (*configuration.DefaultConfig, error) {
	exePath, err := path.GetExecPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get the executable path: %w", err)
	}

	return &configuration.DefaultConfig{
		Title: "Context",
		Parameters: key_value.Empty().
			Set(SrcKey, path.GetPath(exePath, "./deps/.src")).
			Set(BinKey, path.GetPath(exePath, "./deps/.bin")).
			Set(DataKey, path.GetPath(exePath, "./deps/.data")),
	}, nil
}

func newContext(config *configuration.Config) (*Context, error) {
	execPath, err := path.GetExecPath()
	if err != nil {
		return nil, fmt.Errorf("path.GetExecPath: %w", err)
	}
	srcPath := path.GetPath(execPath, config.Engine().GetString(SrcKey))
	dataPath := path.GetPath(execPath, config.Engine().GetString(DataKey))
	binPath := path.GetPath(execPath, config.Engine().GetString(BinKey))

	return &Context{
		Src:  srcPath,
		Bin:  binPath,
		Data: dataPath,
	}, nil
}

// NewDev creates a dev context
func NewDev(config *configuration.Config) (*Context, error) {
	context, err := newContext(config)
	if err != nil {
		return nil, fmt.Errorf("newContext: %w", err)
	}
	context.Type = DevContext
	return context, nil
}

func (context *Context) Paths() []string {
	return []string{context.Data, context.Bin, context.Src}
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
