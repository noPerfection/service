package service

import (
	"fmt"
	"sync"

	"github.com/noPerfection/log"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service/handlers"
	"github.com/noPerfection/service/manager"
	"github.com/noPerfection/service/package_url"
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
	manager         *manager.ProxyManager // manage this proxy from other parts
}

// NewProxy returns a new proxy service.
// Optional parameters are topology config path and manager endpoint.
func NewProxy(name string, params ...interface{}) (*Proxy, error) {
	if name == "" {
		return nil, fmt.Errorf("name argument is required")
	}

	configPath := DefaultConfigPath
	if len(params) > 2 {
		return nil, fmt.Errorf("too many arguments, expected name, config path, and manager endpoint")
	}
	if len(params) > 0 && params[0] != nil {
		configPathArg, ok := params[0].(string)
		if !ok {
			return nil, fmt.Errorf("config path argument must be string")
		}
		if len(configPathArg) > 0 {
			configPath = configPathArg
		}
	}

	topologyHandler, err := newTopologyHandler(configPath)
	if err != nil {
		return nil, fmt.Errorf("topology.NewHandler: %w", err)
	}

	managerEndpoint := manager.DefaultProxyManagerEndpoint(name)
	if len(params) > 1 && params[1] != nil {
		managerEndpointArg, ok := params[1].(message.Endpoint)
		if !ok {
			return nil, fmt.Errorf("manager endpoint argument must be message.Endpoint")
		}
		managerEndpoint = managerEndpointArg
	} else {
		serviceConfig, err := topologyHandler.Service(name)
		if err == nil {
			managerHandler, err := serviceConfig.HandlerByCategory(topology.ServiceManagerCategory)
			if err == nil {
				handler, ok := managerHandler.AsIndependentHandler()
				if ok {
					managerEndpoint = handler.Endpoint
				}
			}
		}
	}

	m, err := manager.NewProxyManager(name, managerEndpoint)
	if err != nil {
		return nil, fmt.Errorf("manager.NewProxyManager: %w", err)
	}

	return &Proxy{
		ProxyHandlers:         handlers.NewProxyHandlers(name),
		WithHardcodedTopology: NewHardcodedTopologies(name),
		topologyHandler:       topologyHandler,
		name:                  name,
		manager:               m,
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

	if proxy.manager != nil {
		if err := proxy.manager.SetLogger(logger); err != nil {
			return fmt.Errorf("manager.SetLogger: %w", err)
		}
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

func (proxy *Proxy) addDefaultServiceToTopology() error {
	serviceConfig, err := proxy.topologyHandler.Service(proxy.name)
	if err == nil {
		return nil
	}

	serviceConfig = config.Service{
		Type:     config.ProxyType,
		Name:     proxy.name,
		Handlers: []config.Handler{},
	}
	if serviceConfig.ModuleUrl == "" {
		moduleURL, err := package_url.FillDefaultModuleURL()
		if err != nil {
			return err
		}
		serviceConfig.ModuleUrl = moduleURL
	}

	if err := proxy.topologyHandler.AddService(serviceConfig); err != nil {
		return fmt.Errorf("topologyHandler.AddService('%s'): %w", proxy.name, err)
	}

	return nil
}

func (proxy *Proxy) addServiceManagerToTopology() error {
	// Proxy topology records only accept proxy handlers. The manager runs
	// in-process via ProxyHandlers, not as a stored independent handler.
	return nil
}

// Start starts the proxy and its service manager.
func (proxy *Proxy) Start() error {
	var err error

	if err = proxy.addDefaultServiceToTopology(); err != nil {
		err = fmt.Errorf("addDefaultServiceToTopology: %w", err)
		goto errOccurred
	}
	if err = proxy.addServiceManagerToTopology(); err != nil {
		err = fmt.Errorf("addServiceManagerToTopology: %w", err)
		goto errOccurred
	}
	if err = proxy.topologyHandler.Start(); err != nil {
		err = fmt.Errorf("topologyHandler.Start(): %w", err)
		goto errOccurred
	}
	if err = proxy.ProxyHandlers.Start(); err != nil {
		err = fmt.Errorf("proxyHandlers.Start: %w", err)
		goto errOccurred
	}

	proxy.blocker = &sync.WaitGroup{}
	proxy.blocker.Add(1)
	proxy.manager.SetSharedBlocker(&proxy.blocker)

	if err = proxy.manager.Start(); err != nil {
		err = fmt.Errorf("proxy.manager.Start: %w", err)
		goto errOccurred
	}

errOccurred:
	if err != nil {
		if proxy.manager != nil && proxy.manager.Running() {
			closeErr := proxy.manager.StopService(proxy.name)
			if closeErr != nil {
				err = fmt.Errorf("%v: manager.StopService: %w", err, closeErr)
			}
		}
	}

	return err
}

func (proxy *Proxy) Stop() error {
	if proxy.manager == nil {
		return proxy.ProxyHandlers.Close()
	}
	if err := proxy.manager.StopService(proxy.name); err != nil {
		return err
	}
	if err := proxy.ProxyHandlers.Close(); err != nil {
		return fmt.Errorf("proxyHandlers.Close: %w", err)
	}
	return nil
}

func (proxy *Proxy) Wait() {
	if proxy.blocker == nil {
		return
	}
	proxy.blocker.Wait()
}
