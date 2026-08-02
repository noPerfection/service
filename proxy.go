package service

import (
	"fmt"
	"sync"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/log"
	"github.com/noPerfection/protocol/handler/npac"
	"github.com/noPerfection/service/handlers"
	"github.com/noPerfection/service/manager"
	"github.com/noPerfection/service/mushroom"
	"github.com/noPerfection/service/package_url"
	"github.com/noPerfection/service/zap"
	"github.com/noPerfection/topology/config"
)

// Proxy keeps the minimal proxy service state.
type Proxy struct {
	*handlers.ProxySetup
	*WithHardcodedTopology
	*TopologyConnection
	name        string
	mushroomURL mushroom.TopologyURL
	blocker     *sync.WaitGroup
	manager     *manager.ProxyManager // manage this proxy from other parts
}

func (proxy *Proxy) dereference() string {
	if proxy.mushroomURL.String() == "" {
		return proxy.name
	}
	return proxy.mushroomURL.AsDereference().String()
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

// NewProxy returns a new proxy service.
func NewProxy(name string) (*Proxy, error) {
	if name == "" {
		return nil, fmt.Errorf("name argument is required")
	}

	return &Proxy{
		ProxySetup:            handlers.NewProxyHandlers(name),
		WithHardcodedTopology: NewHardcodedTopologies(name),
		TopologyConnection:    newTopologyConnection(),
		name:                  name,
	}, nil
}

// EnableLogger toggles the optional proxy logger.
func (proxy *Proxy) EnableLogger(enable bool) error {
	if !enable {
		if err := proxy.ProxySetup.SetLogger(nil); err != nil {
			return fmt.Errorf("proxyHandlers.SetLogger: %w", err)
		}
		return nil
	}

	logger, err := log.New(proxy.name, true)
	if err != nil {
		return fmt.Errorf("log.New(%s): %w", proxy.name, err)
	}
	if err := proxy.ProxySetup.SetLogger(logger); err != nil {
		return fmt.Errorf("proxyHandlers.SetLogger: %w", err)
	}

	if proxy.manager != nil {
		if err := proxy.manager.SetLogger(logger); err != nil {
			return fmt.Errorf("manager.SetLogger: %w", err)
		}
	}

	return nil
}

// Type returns the configuration type for a proxy service.
func (proxy *Proxy) Type() config.Type {
	return config.ProxyType
}

func (proxy *Proxy) addDefaultServiceToTopology() error {
	tp := proxy.topology()
	serviceConfig, err := tp.Service(proxy.dereference())
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

// ensureServiceManager creates the proxy manager from topology configuration.
// When the service record has a manager handler, that endpoint is used;
// otherwise manager.DefaultProxyManagerEndpoint is used.
func (proxy *Proxy) ensureServiceManager() error {
	tp := proxy.topology()
	serviceConfig, err := tp.Service(proxy.dereference())
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", proxy.dereference(), err)
	}

	managerEndpoint := manager.DefaultProxyManagerEndpoint(serviceConfig.Name)
	currentManager, err := serviceConfig.HandlerByCategory(config.ServiceManagerCategory)
	if err == nil {
		handler, ok := currentManager.AsIndependentHandler()
		if ok {
			managerEndpoint = handler.Endpoint
		}
	}

	var secretKey string
	if serviceConfig.Parameters != nil {
		if v, ok := serviceConfig.Parameters[ManagerSecretKeyParameter]; ok {
			secretKey, _ = v.(string)
		}
	}

	m, err := manager.NewProxyManager(serviceConfig.Name, managerEndpoint, secretKey)
	if err != nil {
		return fmt.Errorf("manager.NewProxyManager: %w", err)
	}
	proxy.manager = m
	if err := proxy.addAllowedManagerClients(serviceConfig); err != nil {
		return fmt.Errorf("addAllowedManagerClients: %w", err)
	}

	proxy.manager.RequireWhitelist()

	return nil
}

// Start starts the proxy and its service manager.
func (proxy *Proxy) Start() error {
	var err error
	var topologySnapshot string
	var serviceLink string
	var npacAnyContextPushed bool

	if err = npac.New().Start(); err != nil {
		err = fmt.Errorf("npac.Start: %w", err)
		goto errOccurred
	}
	if err = zap.Start(); err != nil {
		err = fmt.Errorf("zap.Start: %w", err)
		goto errOccurred
	}
	if err = proxy.TopologyConnection.setupTopologyConnection(); err != nil {
		err = fmt.Errorf("setupTopologyConnection: %w", err)
		goto errOccurred
	}

	topologySnapshot, err = proxy.topology().Snapshot()
	if err != nil {
		err = fmt.Errorf("topology.Snapshot: %w", err)
		goto errOccurred
	}
	if err = proxy.WithHardcodedTopology.addHardcodedServicesToTopology(proxy.topology()); err != nil {
		err = fmt.Errorf("addHardcodedServicesToTopology: %w", err)
		goto errOccurred
	}
	if err = proxy.addDefaultServiceToTopology(); err != nil {
		err = fmt.Errorf("addDefaultServiceToTopology: %w", err)
		goto errOccurred
	}
	if err = proxy.WithHardcodedTopology.addHardcodedHandlersToTopology(proxy.topology()); err != nil {
		err = fmt.Errorf("addHardcodedHandlersToTopology: %w", err)
		goto errOccurred
	}
	if err = proxy.WithHardcodedTopology.addHardcodedHandlerDepsToTopology(proxy.topology()); err != nil {
		err = fmt.Errorf("addHardcodedHandlerDepsToTopology: %w", err)
		goto errOccurred
	}
	if err = proxy.WithHardcodedTopology.addHardcodedEndpointsToTopology(proxy.topology()); err != nil {
		err = fmt.Errorf("addHardcodedEndpointsToTopology: %w", err)
		goto errOccurred
	}

	serviceLink, err = proxy.topology().GetLink(proxy.name)
	if err != nil {
		err = fmt.Errorf("topology.GetLink('%s'): %w", proxy.name, err)
		goto errOccurred
	}
	proxy.mushroomURL, err = mushroom.Parse(serviceLink)
	if err != nil {
		err = fmt.Errorf("mushroom.Parse('%s'): %w", serviceLink, err)
		goto errOccurred
	}

	if err = proxy.ensureServiceManager(); err != nil {
		err = fmt.Errorf("ensureServiceManager: %w", err)
		goto errOccurred
	}

	// Managers must have the public keys
	if proxy.manager.PublicKey() == "" {
		err = fmt.Errorf("manager.PublicKey() is empty")
		goto errOccurred
	}

	if err = proxy.allowServiceManager(); err != nil {
		err = fmt.Errorf("allowServiceManager: %w", err)
		goto errOccurred
	}

	if proxy.TopologyConnection.topologyHandler != nil {
		if err = proxy.TopologyConnection.topologyHandler.Start(); err != nil {
			err = fmt.Errorf("topologyHandler.Start(): %w", err)
			goto errOccurred
		}
	}
	if err = proxy.TopologyConnection.ensureTopologyClient(); err != nil {
		err = fmt.Errorf("ensureTopologyClient: %w", err)
		goto errOccurred
	}
	if err = proxy.ProxySetup.Start(serviceLink); err != nil {
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

	if err = proxy.manager.NpacPushAnyContext(proxy.Start); err != nil {
		err = fmt.Errorf("manager.NpacPushAnyContext: %w", err)
		goto errOccurred
	}
	npacAnyContextPushed = true
	defer func() {
		if npacAnyContextPushed && proxy.manager != nil {
			_ = proxy.manager.NpacPopAnyContext(proxy.Start)
		}
	}()

	if err = proxy.manager.Handshake(); err != nil {
		err = fmt.Errorf("manager.Handshake: %w", err)
		goto errOccurred
	}

errOccurred:
	if err != nil {
		if topologySnapshot != "" {
			if rollbackErr := proxy.topology().Rollback(topologySnapshot); rollbackErr != nil {
				err = fmt.Errorf("%w: topology.Rollback: %v", err, rollbackErr)
			}
		}
		if topologyCloseErr := proxy.TopologyConnection.closeTopologyClient(); topologyCloseErr != nil {
			err = fmt.Errorf("%w: closeTopologyClient: %w", err, topologyCloseErr)
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

func (proxy *Proxy) allowServiceManager() error {
	tp := proxy.topology()
	serviceConfig, err := tp.Service(proxy.dereference())
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", proxy.dereference(), err)
	}
	serviceLink := proxy.mushroomURL.String()

	managerLink, err := mushroom.As(serviceLink, config.ServiceManagerCategory)
	if err != nil {
		return fmt.Errorf("mushroom.As(%q, %q): %w", serviceLink, config.ServiceManagerCategory, err)
	}
	publicKey := proxy.manager.PublicKey()

	if serviceConfig.Parameters == nil {
		serviceConfig.Parameters = datatype.New()
	}
	if existing, _ := serviceConfig.Parameters[manager.ManagerPublicKeyParam].(string); existing != publicKey {
		serviceConfig.Parameters[manager.ManagerPublicKeyParam] = publicKey
		if err := tp.SetService(serviceConfig); err != nil {
			return fmt.Errorf("topology.SetService('%s') store public key: %w", proxy.name, err)
		}
	}

	depServiceURLs := make(map[string]struct{})

	for _, dep := range serviceConfig.HandlerDeps {
		for _, u := range dep.Proxies {
			link, err := tp.GetLink(u)
			if err != nil {
				return fmt.Errorf("topology.GetLink('%s'): %w", u, err)
			}
			depServiceURLs[link] = struct{}{}
		}
		for _, u := range dep.Extensions {
			link, err := tp.GetLink(u)
			if err != nil {
				return fmt.Errorf("topology.GetLink('%s'): %w", u, err)
			}
			depServiceURLs[link] = struct{}{}
		}
	}
	for _, variant := range serviceConfig.Handlers {
		handler, ok := variant.AsIndependentHandler()
		if !ok {
			continue
		}
		for _, dep := range handler.CommandDeps {
			for _, u := range dep.Proxies {
				link, err := tp.GetLink(u)
				if err != nil {
					return fmt.Errorf("topology.GetLink('%s'): %w", u, err)
				}
				depServiceURLs[link] = struct{}{}
			}
			for _, u := range dep.Extensions {
				link, err := tp.GetLink(u)
				if err != nil {
					return fmt.Errorf("topology.GetLink('%s'): %w", u, err)
				}
				depServiceURLs[link] = struct{}{}
			}
		}
	}

	for svcURL := range depServiceURLs {
		mushroomURL, err := mushroom.Parse(svcURL)
		if err != nil {
			return fmt.Errorf("mushroom.Parse('%s'): %w", svcURL, err)
		}
		depService, err := tp.Service(mushroomURL.AsDereference().String())
		if err != nil {
			return fmt.Errorf("topology.Service('%s'): %w", svcURL, err)
		}
		if !mushroom.IsAllowedPublicKeyMatch(&depService, managerLink, managerLink.ResourcePublicKey()) {
			continue
		}
		mushroom.AddAllowedPublicKey(&depService, managerLink, managerLink.ResourcePublicKey())
		if err := tp.SetService(depService); err != nil {
			return fmt.Errorf("topology.SetService('%s'): %w", depService.Name, err)
		}
	}

	return nil
}

func (proxy *Proxy) addAllowedManagerClients(serviceConfig config.Service) error {
	entryMap := mushroom.AllowedKeyValues(&serviceConfig, config.ServiceManagerCategory)
	if entryMap == nil {
		return nil
	}

	for url, pubKeyVal := range entryMap {
		pubKey, ok := pubKeyVal.(string)
		if !ok || pubKey == "" {
			continue
		}
		mushroomURL, err := mushroom.Parse(url)
		if err != nil {
			return fmt.Errorf("entryMap.mushroom.Parse('%s'): %w", url, err)
		}
		zap.AuthCurveAdd(proxy.mushroomURL.As(mushroom.HANDLER).String(), pubKey, mushroomURL.As(mushroom.HANDLER))
	}

	return nil
}

func (proxy *Proxy) Stop() error {
	if proxy.manager != nil && !proxy.manager.Running() {
		return nil
	}

	if err := proxy.TopologyConnection.closeTopologyClient(); err != nil {
		return fmt.Errorf("closeTopologyClient: %w", err)
	}
	if proxy.manager == nil {
		return proxy.ProxySetup.Close()
	}
	if err := proxy.manager.StopService(proxy.name); err != nil {
		return err
	}
	if err := proxy.ProxySetup.Close(); err != nil {
		return fmt.Errorf("proxyHandlers.Close: %w", err)
	}
	return nil
}

func (proxy *Proxy) Wait() {
	waitForShutdown(proxy.blocker, proxy.Stop)
}
