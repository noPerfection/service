package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/noPerfection/log"
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
	topologyClient  *topology.Client
	name            string
	blocker         *sync.WaitGroup
	manager         *manager.ProxyManager // manage this proxy from other parts
}

// Follows pkg:golang/github.com/noPerfection/service?object=Service&root=no_perfection.go
func (proxy *Proxy) isService() {}

func (proxy *Proxy) AsIndependent() (*Independent, bool) {
	return nil, false
}

func (proxy *Proxy) AsProxy() (*Proxy, bool) {
	if proxy == nil {
		return nil, false
	}
	return proxy, true
}

func (proxy *Proxy) AsExtension() (*Extension, bool) {
	return nil, false
}

func (proxy *Proxy) topology() topology.TopologyInterface {
	if proxy == nil {
		return nil
	}
	if proxy.topologyClient != nil {
		return proxy.topologyClient
	}
	return proxy.topologyHandler
}

// NewProxy returns a new proxy service.
func NewProxy(name string) (*Proxy, error) {
	if name == "" {
		return nil, fmt.Errorf("name argument is required")
	}

	return &Proxy{
		ProxyHandlers:         handlers.NewProxyHandlers(name),
		WithHardcodedTopology: NewHardcodedTopologies(name),
		name:                  name,
	}, nil
}

// SetTopologyParams configures the local topology handler before Start.
// Supported keys: "filepath" — topology JSON path (default DefaultConfigPath).
func (proxy *Proxy) SetTopologyParams(params map[string]any) error {
	if proxy == nil {
		return fmt.Errorf("proxy is nil")
	}
	if proxy.topologyHandler != nil {
		return fmt.Errorf("topology handler already configured")
	}
	if params == nil {
		params = map[string]any{}
	}
	for key := range params {
		if key != TopologyParamFilepath {
			return fmt.Errorf("unsupported topology param %q", key)
		}
	}
	configPath := DefaultConfigPath
	if v, ok := params[TopologyParamFilepath]; ok && v != nil {
		filepath, ok := v.(string)
		if !ok {
			return fmt.Errorf("topology param %q must be string", TopologyParamFilepath)
		}
		if filepath != "" {
			configPath = filepath
		}
	}
	h, err := newTopologyHandler(configPath)
	if err != nil {
		return err
	}
	proxy.topologyHandler = h
	return nil
}

func (proxy *Proxy) ensureTopologyHandler() error {
	if proxy.topologyHandler != nil {
		return nil
	}
	h, err := newTopologyHandler(DefaultConfigPath)
	if err != nil {
		return err
	}
	proxy.topologyHandler = h
	return nil
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
	tp := proxy.topology()
	serviceConfig, err := tp.Service(proxy.name)
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

	if err := tp.AddService(serviceConfig); err != nil {
		return fmt.Errorf("topology.AddService('%s'): %w", proxy.name, err)
	}

	return nil
}

func (proxy *Proxy) addHardcodedServicesToTopology() error {
	if proxy == nil || proxy.WithHardcodedTopology == nil {
		return fmt.Errorf("proxy or WithHardcodedTopology is nil")
	}
	tp := proxy.topology()

	for mushroomURL, serviceConfig := range proxy.serviceConfigs {
		parent := serviceParentURL(mushroomURL)
		_, err := tp.Service(mushroomURL)
		if err != nil {
			if err := tp.AddService(serviceConfig, parent...); err != nil {
				return fmt.Errorf("topology.AddService(%q): %w", mushroomURL, err)
			}
			continue
		}

		if err := tp.SetService(serviceConfig, parent...); err != nil {
			return fmt.Errorf("topology.SetService(%q): %w", mushroomURL, err)
		}
	}

	return nil
}

func (proxy *Proxy) addHardcodedHandlersToTopology() error {
	if proxy == nil || proxy.WithHardcodedTopology == nil {
		return fmt.Errorf("proxy or WithHardcodedTopology is nil")
	}
	handlers, ok := proxy.handlerConfigs[proxy.name]
	if !ok || len(handlers) == 0 {
		return nil
	}

	tp := proxy.topology()
	serviceConfig, err := tp.Service(proxy.name)
	if err != nil {
		return fmt.Errorf("hardcoded handlers for %q not found in topology: %w", proxy.name, err)
	}

	for _, handler := range handlers {
		if proxyHandler, ok := handler.AsProxyHandler(); ok {
			handler = normalizeProxyHandlerOutbounds(proxyHandler)
		}
		serviceConfig.SetHandler(handler, true)
	}
	if err := tp.SetService(serviceConfig, serviceParentURL(proxy.name)...); err != nil {
		return fmt.Errorf("topology.SetService(%q): %w", proxy.name, err)
	}

	return nil
}

func (proxy *Proxy) addHardcodedEndpointsToTopology() error {
	if proxy == nil || proxy.WithHardcodedTopology == nil {
		return fmt.Errorf("proxy or WithHardcodedTopology is nil")
	}
	tp := proxy.topology()

	for mushroomURL, endpointsByHandler := range proxy.endpoints {
		serviceConfig, err := tp.Service(mushroomURL)
		if err != nil {
			return fmt.Errorf("hardcoded endpoints for %q not found in topology: %w", mushroomURL, err)
		}

		for handlerCategory, endpoint := range endpointsByHandler {
			handlerVariant, err := serviceConfig.HandlerByCategory(handlerCategory)
			if err != nil {
				return fmt.Errorf("hardcoded endpoints handler '%s' in service %q: %w", handlerCategory, mushroomURL, err)
			}

			serviceConfig.SetHandler(setHandlerEndpoint(handlerVariant, endpoint), true)
		}
		if err := tp.SetService(serviceConfig, serviceParentURL(mushroomURL)...); err != nil {
			return fmt.Errorf("topology.SetService(%q): %w", mushroomURL, err)
		}
	}

	return nil
}

// ensureServiceManager creates the proxy manager from topology configuration.
// When the service record has a manager handler, that endpoint is used;
// otherwise manager.DefaultProxyManagerEndpoint is used.
func (proxy *Proxy) ensureServiceManager() error {
	tp := proxy.topology()
	serviceConfig, err := tp.Service(proxy.name)
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", proxy.name, err)
	}

	managerEndpoint := manager.DefaultProxyManagerEndpoint(proxy.name)
	currentManager, err := serviceConfig.HandlerByCategory(topology.ServiceManagerCategory)
	if err == nil {
		handler, ok := currentManager.AsIndependentHandler()
		if ok {
			managerEndpoint = handler.Endpoint
		}
	}

	m, err := manager.NewProxyManager(proxy.name, managerEndpoint)
	if err != nil {
		return fmt.Errorf("manager.NewProxyManager: %w", err)
	}
	proxy.manager = m

	return nil
}

// Start starts the proxy and its service manager.
func (proxy *Proxy) Start() error {
	var err error
	var topologySnapshot string

	if err = proxy.connectTopologyClientIfRunning(); err != nil {
		err = fmt.Errorf("connectTopologyClientIfRunning: %w", err)
		goto errOccurred
	}
	if err = proxy.ensureTopologyHandler(); err != nil {
		err = fmt.Errorf("ensureTopologyHandler: %w", err)
		goto errOccurred
	}
	topologySnapshot, err = proxy.topology().Snapshot()
	if err != nil {
		err = fmt.Errorf("topology.Snapshot: %w", err)
		goto errOccurred
	}
	if err = proxy.addHardcodedServicesToTopology(); err != nil {
		err = fmt.Errorf("addHardcodedServicesToTopology: %w", err)
		goto errOccurred
	}
	if err = proxy.addDefaultServiceToTopology(); err != nil {
		err = fmt.Errorf("addDefaultServiceToTopology: %w", err)
		goto errOccurred
	}
	if err = proxy.addHardcodedHandlersToTopology(); err != nil {
		err = fmt.Errorf("addHardcodedHandlersToTopology: %w", err)
		goto errOccurred
	}
	if err = proxy.addHardcodedEndpointsToTopology(); err != nil {
		err = fmt.Errorf("addHardcodedEndpointsToTopology: %w", err)
		goto errOccurred
	}
	if err = proxy.ensureServiceManager(); err != nil {
		err = fmt.Errorf("ensureServiceManager: %w", err)
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
		if topologySnapshot != "" {
			if rollbackErr := proxy.topology().Rollback(topologySnapshot); rollbackErr != nil {
				err = fmt.Errorf("%w: topology.Rollback: %v", err, rollbackErr)
			}
		}
		if proxy.topologyClient != nil {
			_ = proxy.topologyClient.Close()
			proxy.topologyClient = nil
		}
		if proxy.manager != nil && proxy.manager.Running() {
			closeErr := proxy.manager.StopService(proxy.name)
			if closeErr != nil {
				err = fmt.Errorf("%v: manager.StopService: %w", err, closeErr)
			}
		}
	}

	return err
}

func (proxy *Proxy) connectTopologyClientIfRunning() error {
	if proxy == nil || proxy.topologyClient != nil {
		return nil
	}
	client, err := topology.NewClient()
	if err != nil {
		return fmt.Errorf("topology.NewClient: %w", err)
	}
	client.Timeout(50 * time.Millisecond)
	client.Attempt(1)
	running, err := client.IsRunning()
	if err != nil || !running {
		_ = client.Close()
		return nil
	}
	client.Attempt(2)
	proxy.topologyClient = client
	return nil
}

func (proxy *Proxy) Stop() error {
	if proxy.topologyClient != nil {
		_ = proxy.topologyClient.Close()
		proxy.topologyClient = nil
	}
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
