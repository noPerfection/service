package context

type Type = string

// DevContext indicates that all dependency proxies are in the local machine
const DevContext Type = "development"

// DefaultContext indicates that all dependencies are in any machine.
// It's unspecified.
const DefaultContext Type = "default"

// A Context handles the configuration of the contexts
type Context struct {
	Type Type   `json:"CONTEXT_TYPE"`
	Src  string `json:"SERVICE_DEPS_SRC"`
	Bin  string `json:"SERVICE_DEPS_BIN"`
	Data string `json:"SERVICE_DEPS_DATA"`
	url  string
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
