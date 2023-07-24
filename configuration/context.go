package configuration

import "path"

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
	Type ContextType
	Src  string
	Bin  string
	Data string
}

func initContext(config *Config) {
	exePath, err := GetCurrentPath()
	if err != nil {
		config.logger.Fatal("failed to get the executable path", "error", err)
	}

	config.viper.SetDefault(SrcKey, path.Join(exePath, "deps", ".src"))
	config.viper.SetDefault(BinKey, path.Join(exePath, "deps", ".bin"))
	config.viper.SetDefault(DataKey, path.Join(exePath, "deps", ".data"))
}

func newContext(config *Config) *Context {
	return &Context{
		Src:  config.viper.GetString(SrcKey),
		Bin:  config.viper.GetString(BinKey),
		Data: config.viper.GetString(DataKey),
	}
}

func setDevContext(config *Config) {
	context := newContext(config)
	context.Type = DevContext

	config.Context = context
}

func (context *Context) Host() string {
	if context.Type == DevContext {
		return "localhost"
	}
	return "0.0.0.0"
}
