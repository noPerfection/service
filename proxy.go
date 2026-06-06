package service

import (
	"fmt"
	"sync"

	"github.com/noPerfection/log"
	"github.com/noPerfection/service/handlers"
	"github.com/noPerfection/topology"
	"github.com/noPerfection/topology/config"
)

// Proxy keeps the minimal proxy service state.
type Proxy struct {
	*handlers.ProxyHandlers
	*WithHardcodedTopology
	topologyHandler *topology.Handler // topology handles the configuration and dependencies
	name            string
	blocker         *sync.WaitGroup
}

// NewProxy returns a new proxy service.
// Optional parameters are name and topology config path.
func NewProxy(params ...interface{}) (*Proxy, error) {
	name := DefaultName
	configPath := DefaultConfigPath

	if len(params) > 2 {
		return nil, fmt.Errorf("too many arguments, expected name and config path")
	}
	if len(params) > 0 && params[0] != nil {
		nameArg, ok := params[0].(string)
		if !ok {
			return nil, fmt.Errorf("name argument must be string")
		}
		if len(nameArg) > 0 {
			name = nameArg
		}
	}
	if len(params) > 1 && params[1] != nil {
		configPathArg, ok := params[1].(string)
		if !ok {
			return nil, fmt.Errorf("config path argument must be string")
		}
		if len(configPathArg) > 0 {
			configPath = configPathArg
		}
	}

	topologyHandler, err := topology.NewHandler(configPath)
	if err != nil {
		return nil, fmt.Errorf("topology.NewHandler: %w", err)
	}

	return &Proxy{
		ProxyHandlers:         handlers.NewProxyHandlers(name),
		WithHardcodedTopology: NewHardcodedTopologies(name),
		topologyHandler:       topologyHandler,
		name:                  name,
	}, nil
}

// EnableLogger toggles the optional proxy logger.
func (proxy *Proxy) EnableLogger(enable bool) error {
	if !enable {
		if err := proxy.ProxyHandlers.SetLogger(nil); err != nil {
			return fmt.Errorf("proxyHandlers.SetLogger: %w", err)
		}
		return nil
	}

	logger, err := log.New(proxy.name, true)
	if err != nil {
		return fmt.Errorf("log.New(%s): %w", proxy.name, err)
	}
	if err := proxy.ProxyHandlers.SetLogger(logger); err != nil {
		return fmt.Errorf("proxyHandlers.SetLogger: %w", err)
	}

	return nil
}

// Name returns the unique name of the proxy.
func (proxy *Proxy) Name() string {
	return proxy.name
}

// Type returns the configuration type for a proxy service.
func (proxy *Proxy) Type() config.Type {
	return config.ProxyType
}

// Start starts the proxy placeholder.
func (proxy *Proxy) Start() error {
	if err := proxy.ProxyHandlers.Start(); err != nil {
		return fmt.Errorf("proxyHandlers.Start: %w", err)
	}

	proxy.blocker = &sync.WaitGroup{}
	proxy.blocker.Add(1)
	return nil
}

func (proxy *Proxy) Stop() error {
	return fmt.Errorf("todo not implemented")
}

func (proxy *Proxy) Wait() {
	if proxy.blocker == nil {
		return
	}
	proxy.blocker.Wait()
}
