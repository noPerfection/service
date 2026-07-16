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
	"github.com/noPerfection/topology"
	"github.com/noPerfection/topology/config"
)

// Extension keeps all necessary parameters of the independent service.
type Extension struct {
	*handlers.Setup
	*WithHardcodedTopology
	*TopologyConnection
	rawMushroomURL string
	mushroomURL    mushroom.TopologyURL
	blocker        *sync.WaitGroup
	manager        *manager.Manager // manage this service from other parts
	logger         *log.Logger
}

// Follows pkg:golang/github.com/noPerfection/service?object=Service&root=no_perfection.go
func (extension *Extension) isService() {}

func (extension *Extension) AsIndependent() (*Independent, bool) {
	return nil, false
}

func (extension *Extension) AsProxy() (*Proxy, bool) {
	return nil, false
}

func (extension *Extension) AsExtension() (*Extension, bool) {
	if extension == nil {
		return nil, false
	}
	return extension, true
}

// NewExt returns an extension service instance.
//
// Optional parameter:
//
//  1. rawMushroomURL — service identity in the configuration. A plain symbol is treated as the
//     service name at the root of the topology (e.g. "main" → services[name:main]). Full
//     mushroom paths are accepted but not validated yet.
//
// Use SetTopologyParams before Start to configure the topology JSON path.
//
//	// Root service "main".
//	app, err := NewExt("main")
func NewExt(params ...any) (*Extension, error) {
	mushroomURL := DefaultName

	if len(params) > 1 {
		return nil, fmt.Errorf("too many arguments, expected at most one service name")
	}

	if len(params) > 0 && params[0] != nil {
		mushroomUrlArg, ok := params[0].(string)
		if !ok {
			return nil, fmt.Errorf("name argument must be string")
		}
		if len(mushroomUrlArg) > 0 {
			mushroomURL = mushroomUrlArg
		}
	}

	independent := &Extension{
		Setup:                 handlers.NewSetup(),
		WithHardcodedTopology: NewHardcodedTopologies(mushroomURL),
		TopologyConnection:    newTopologyConnection(),
		rawMushroomURL:        mushroomURL,
		logger:                nil,
	}

	return independent, nil
}

// EnableLogger toggles the optional service logger.
func (independent *Extension) EnableLogger(enable bool) error {
	if !enable {
		if err := independent.Setup.SetLogger(nil); err != nil {
			return fmt.Errorf("handlers.SetLogger: %w", err)
		}
		if independent.manager != nil {
			if err := independent.manager.SetLogger(nil); err != nil {
				return fmt.Errorf("manager.SetLogger: %w", err)
			}
		}
		independent.logger = nil
		return nil
	}

	logger, err := log.New(independent.rawMushroomURL, true)
	if err != nil {
		return fmt.Errorf("log.New(%s): %w", independent.rawMushroomURL, err)
	}
	if err := independent.Setup.SetLogger(logger); err != nil {
		return fmt.Errorf("handlers.SetLogger: %w", err)
	}

	if independent.manager != nil {
		if err := independent.manager.SetLogger(logger); err != nil {
			return fmt.Errorf("manager.SetLogger: %w", err)
		}
	}

	independent.logger = logger
	return nil
}

// addDefaultServiceToTopology adds the default service config
// if no config was given for this service.
func (independent *Extension) addDefaultServiceToTopology() error {
	tp := independent.topology()
	serviceConfig, err := tp.Service(independent.mushroomURL.AsDereference().String())
	if err == nil {
		return nil
	}

	serviceConfig = config.Service{
		Type:     config.ExtensionType,
		Name:     independent.rawMushroomURL,
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
		return fmt.Errorf("topology.AddService('%s'): %w", independent.mushroomURL, err)
	}

	return nil
}

// addDefaultHandlerToTopology adds the default handler when no handlers exist.
// Unless there are handlers set by you or others
func (independent *Extension) addDefaultHandlerToTopology() error {
	tp := independent.topology()
	serviceConfig, err := tp.Service(independent.mushroomURL.AsDereference().String())
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", independent.mushroomURL, err)
	}
	if len(serviceConfig.Handlers) > 0 {
		return nil
	}

	_, err = serviceConfig.HandlerByCategory(handlers.DefaultHandlerCategory)
	// No error indicates the default handler already exists
	if err == nil {
		return nil
	}

	defaultHandler := config.ExtensionHandler{
		IndependentHandler: config.IndependentHandler{
			Category: handlers.DefaultHandlerCategory,
			Endpoint: handlers.DefaultHandlerEndpoint,
			Type:     config.ReplierType,
		},
	}
	serviceConfig.Handlers = []config.Handler{defaultHandler}
	if err := tp.SetService(serviceConfig); err != nil {
		return fmt.Errorf("topology.SetService('%s'): %w", independent.mushroomURL, err)
	}

	return nil
}

// ensureServiceManager creates the service manager from topology configuration.
// When the service record has a manager handler, that endpoint is used;
// otherwise manager.DefaultExtensionManagerEndpoint is used.
func (independent *Extension) ensureServiceManager() error {
	tp := independent.topology()
	serviceConfig, err := tp.Service(independent.mushroomURL.AsDereference().String())
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", independent.mushroomURL, err)
	}

	managerEndpoint := manager.DefaultExtensionManagerEndpoint(independent.mushroomURL.AsDereference().String())
	currentManager, err := serviceConfig.HandlerByCategory(config.ServiceManagerCategory)
	if err == nil {
		handler := currentManager.(config.IndependentHandler)
		managerEndpoint = handler.Endpoint
	}

	var secretKey string
	if serviceConfig.Parameters != nil {
		if v, ok := serviceConfig.Parameters[ManagerSecretKeyParameter]; ok {
			secretKey, _ = v.(string)
		}
	}

	m, err := manager.New(independent.mushroomURL, managerEndpoint, secretKey)
	if err != nil {
		return fmt.Errorf("manager.New: %w", err)
	}
	independent.manager = m
	if err := independent.manager.SetLogger(independent.logger); err != nil {
		return fmt.Errorf("manager.SetLogger: %w", err)
	}

	if err := independent.addAllowedManagerClients(serviceConfig.Parameters); err != nil {
		return fmt.Errorf("addAllowedManagerClients: %w", err)
	}

	return nil
}

// addTopologyHandlersToHandlers adds the handlers to the handlers list.
// Except for the Service Manager category, any handler defined in the topology is
// registered in the handlers package for launching them.
func (independent *Extension) addTopologyHandlersToHandlers() error {
	tp := independent.topology()
	serviceConfig, err := tp.Service(independent.mushroomURL.AsDereference().String())
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", independent.mushroomURL, err)
	}

	for _, configuredVariant := range serviceConfig.Handlers {
		configured, ok := configuredVariant.AsIndependentHandler()
		if !ok {
			continue
		}
		if configured.Category == config.ServiceManagerCategory {
			continue
		}

		handler, err := newHandler(configured.Type)
		if err != nil {
			return fmt.Errorf("newTopologyHandler('%s'): %w", configured.Category, err)
		}
		handler.SetEndpoint(configured.Endpoint)
		if err := independent.Setup.SetHandler(configured.Category, handler); err != nil {
			return fmt.Errorf("handlers.SetHandler('%s'): %w", configured.Category, err)
		}
	}

	return nil
}

// Start the service.
//
// Requires at least one handler.
func (independent *Extension) Start() error {
	var err error
	var inprocServices int
	var topologySnapshot string
	var serviceLink string
	var tp topology.TopologyInterface
	if err = npac.New().Start(); err != nil {
		err = fmt.Errorf("npac.Start: %w", err)
		goto errOccurred
	}
	if err = independent.TopologyConnection.setupTopologyConnection(); err != nil {
		err = fmt.Errorf("setupTopologyConnection: %w", err)
		goto errOccurred
	}
	serviceLink, err = independent.topology().GetLink(independent.rawMushroomURL)
	if err != nil {
		err = fmt.Errorf("topology.GetLink('%s'): %w", independent.rawMushroomURL, err)
		goto errOccurred
	} else {
		independent.mushroomURL, err = mushroom.New(serviceLink)
		if err != nil {
			err = fmt.Errorf("mushroom.New('%s'): %w", serviceLink, err)
			goto errOccurred
		}
	}

	topologySnapshot, err = independent.topology().Snapshot()
	if err != nil {
		err = fmt.Errorf("topology.Snapshot: %w", err)
		goto errOccurred
	}
	if err = independent.WithHardcodedTopology.addHardcodedServicesToTopology(independent.topology()); err != nil {
		err = fmt.Errorf("addHardcodedServicesToTopology: %w", err)
		goto errOccurred
	}
	if err = independent.addDefaultServiceToTopology(); err != nil {
		err = fmt.Errorf("addDefaultServiceToTopology: %w", err)
		goto errOccurred
	}
	if err = independent.WithHardcodedTopology.addHardcodedHandlersToTopology(independent.topology()); err != nil {
		err = fmt.Errorf("addHardcodedHandlersToTopology: %w", err)
		goto errOccurred
	}
	if err = independent.addDefaultHandlerToTopology(); err != nil {
		err = fmt.Errorf("addDefaultHandlerToTopology: %w", err)
		goto errOccurred
	}

	if err = independent.WithHardcodedTopology.addHardcodedHandlerDepsToTopology(independent.topology()); err != nil {
		err = fmt.Errorf("addHardcodedHandlerDepsToTopology: %w", err)
		goto errOccurred
	}
	if err = independent.WithHardcodedTopology.addHardcodedServiceParamsToTopology(independent.topology()); err != nil {
		err = fmt.Errorf("addHardcodedServiceParamsToTopology: %w", err)
		goto errOccurred
	}
	if err = independent.WithHardcodedTopology.addHardcodedEndpointsToTopology(independent.topology()); err != nil {
		err = fmt.Errorf("addHardcodedEndpointsToTopology: %w", err)
		goto errOccurred
	}

	if err = independent.ensureServiceManager(); err != nil {
		err = fmt.Errorf("ensureServiceManager: %w", err)
		goto errOccurred
	}

	if err = independent.WithHardcodedTopology.addHardcodedCommandDepsToTopology(independent.topology()); err != nil {
		err = fmt.Errorf("addHardcodedCommandDepsToTopology: %w", err)
		goto errOccurred
	}

	if err = independent.addTopologyHandlersToHandlers(); err != nil {
		err = fmt.Errorf("addTopologyHandlers: %w", err)
		goto errOccurred
	}

	tp = independent.topology()
	if err = tp.ValidateProtocolOrder(independent.mushroomURL.AsDereference().String()); err != nil {
		err = fmt.Errorf("topology.ValidateProtocolOrder: %w", err)
		goto errOccurred
	}
	if err = tp.ValidateInprocServiceManagers(); err != nil {
		err = fmt.Errorf("topology.ValidateInprocServiceManagers: %w", err)
		goto errOccurred
	}
	if inprocServices, err = tp.InprocessDepNumber(independent.mushroomURL.AsDereference().String()); err != nil {
		err = fmt.Errorf("topology.InprocessDepNumber: %w", err)
		goto errOccurred
	}

	// Managers must have the public keys
	if independent.manager.PublicKey() == "" {
		err = fmt.Errorf("manager.PublicKey() is empty")
		goto errOccurred
	}

	if err = independent.allowServiceManager(); err != nil {
		err = fmt.Errorf("allowServiceManager: %w", err)
		goto errOccurred
	}

	if independent.TopologyConnection.topologyHandler != nil {
		if err = independent.TopologyConnection.topologyHandler.Start(); err != nil {
			err = fmt.Errorf("topologyHandler.Start(): %w", err)
			goto errOccurred
		}
	}
	if err = independent.TopologyConnection.ensureTopologyClient(); err != nil {
		err = fmt.Errorf("ensureTopologyClient: %w", err)
		goto errOccurred
	}
	if err = independent.Setup.Start(independent.mushroomURL); err != nil {
		err = fmt.Errorf("handlers.Start: %w", err)
		goto errOccurred
	}

	independent.blocker = &sync.WaitGroup{}
	independent.blocker.Add(1)

	independent.manager.SetSharedBlocker(&independent.blocker)
	if err = independent.manager.Start(); err != nil {
		err = fmt.Errorf("service.manager.Start: %w", err)
		goto errOccurred
	}

	if err = independent.syncCommandOutbounds(); err != nil {
		err = fmt.Errorf("syncCommandOutbounds: %w", err)
		goto errOccurred
	}
	if err = independent.syncHandlerDepOutbounds(); err != nil {
		err = fmt.Errorf("syncHandlerDepOutbounds: %w", err)
		goto errOccurred
	}
	if inprocServices > 0 {
		fmt.Printf("todo: implement setupInproc() for extension only if its not running on main file\n")
	}
	if err = independent.startIpcServices(); err != nil {
		err = fmt.Errorf("startIpcServices: %w", err)
		goto errOccurred
	}
	// Wait for all IPC deps concurrently, reloading config on each probe so
	// that public keys written by newly started services are discovered.
	if err = independent.waitServicesRunning(); err != nil {
		err = fmt.Errorf("waitServicesRunning: %w", err)
		goto errOccurred
	}

errOccurred:
	if err != nil {
		if topologySnapshot != "" {
			if rollbackErr := independent.topology().Rollback(topologySnapshot); rollbackErr != nil {
				err = fmt.Errorf("%w: topology.Rollback: %v", err, rollbackErr)
			}
		}
		if topologyCloseErr := independent.TopologyConnection.closeTopologyClient(); topologyCloseErr != nil {
			err = fmt.Errorf("%w: closeTopologyClient: %w", err, topologyCloseErr)
		}
		if independent.manager != nil && independent.manager.Running() {
			closeErr := independent.manager.StopService(independent.mushroomURL.AsDereference().String())
			if closeErr != nil {
				err = fmt.Errorf("%v: manager.StopService: %w", err, closeErr)
			}
		}
	}

	return err
}

func (independent *Extension) syncHandlerDepOutbounds() error {
	tp := independent.topology()
	serviceConfig, err := tp.Service(independent.mushroomURL.AsDereference().String())
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", independent.mushroomURL, err)
	}
	if len(serviceConfig.HandlerDeps) == 0 {
		return nil
	}

	for depIndex := range serviceConfig.HandlerDeps {
		dep := &serviceConfig.HandlerDeps[depIndex]
		if len(dep.Proxies) == 0 {
			continue
		}

		handlerVariant, err := serviceConfig.HandlerByCategory(dep.Name)
		if err != nil {
			return fmt.Errorf("handler dep %q: %w", dep.Name, err)
		}
		handler, ok := handlerVariant.AsIndependentHandler()
		if !ok {
			return fmt.Errorf("handler dep %q is not an independent handler", dep.Name)
		}
		routes, err := independent.Setup.RouteCommands(dep.Name)
		if err != nil {
			return fmt.Errorf("handler dep %q route commands: %w", dep.Name, err)
		}
		if len(routes) == 0 {
			continue
		}

		for proxyIndex := range dep.Proxies {
			proxyURL := dep.Proxies[proxyIndex]
			outbound, commandOutbounds, err := independent.handlerDepProxyOutboundTargets(handler, dep.Proxies, proxyIndex, routes)
			if err != nil {
				return fmt.Errorf("handler %q proxy %q outbound: %w", dep.Name, proxyURL, err)
			}
			if err := independent.syncHandlerDepProxyOutbounds(routes, proxyURL, outbound, commandOutbounds); err != nil {
				return fmt.Errorf("handler %q proxy %q: %w", dep.Name, proxyURL, err)
			}
		}
	}

	return nil
}

// startIpcServices starts IPC services this service depends on.
func (independent *Extension) startIpcServices() error {
	tp := independent.topology()
	serviceConfig, err := tp.Service(independent.mushroomURL.AsDereference().String())
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", independent.mushroomURL, err)
	}

	startedRefs := make(map[string]struct{})
	return independent.startIpcServicesFor(serviceConfig, startedRefs)
}

func (independent *Extension) startIpcServicesFor(serviceConfig config.Service, startedRefs map[string]struct{}) error {
	tp := independent.topology()
	for _, dep := range serviceConfig.HandlerDeps {
		for _, proxy := range dep.Proxies {
			link, err := tp.GetLink(proxy)
			if err != nil {
				return fmt.Errorf("topology.GetLink('%s'): %w", proxy, err)
			}
			if err := independent.startIpcService(link, startedRefs); err != nil {
				return fmt.Errorf("handler dep %q proxy %q: %w", dep.Name, proxy, err)
			}
		}
		for _, extension := range dep.Extensions {
			link, err := tp.GetLink(extension)
			if err != nil {
				return fmt.Errorf("topology.GetLink('%s'): %w", extension, err)
			}
			if err := independent.startIpcService(link, startedRefs); err != nil {
				return fmt.Errorf("handler dep %q extension %q: %w", dep.Name, extension, err)
			}
		}
	}

	for _, variant := range serviceConfig.Handlers {
		handler, ok := variant.AsIndependentHandler()
		if !ok {
			continue
		}
		for _, dep := range handler.CommandDeps {
			for _, proxy := range dep.Proxies {
				link, err := tp.GetLink(proxy)
				if err != nil {
					return fmt.Errorf("topology.GetLink('%s'): %w", proxy, err)
				}
				if err := independent.startIpcService(link, startedRefs); err != nil {
					return fmt.Errorf("handler %q command %q proxy %q: %w", handler.Category, dep.Name, proxy, err)
				}
			}
			for _, extension := range dep.Extensions {
				link, err := tp.GetLink(extension)
				if err != nil {
					return fmt.Errorf("topology.GetLink('%s'): %w", extension, err)
				}
				if err := independent.startIpcService(link, startedRefs); err != nil {
					return fmt.Errorf("handler %q command %q extension %q: %w", handler.Category, dep.Name, extension, err)
				}
			}
		}
	}

	return nil
}

// waitServicesRunning waits for every direct dep service (IPC and inproc) to
// become running, probing them all in parallel.
// attempts=10 with the IPC probe timeout of ~100ms gives ~1s total per dep.
func (independent *Extension) waitServicesRunning() error {
	tp := independent.topology()
	serviceConfig, err := tp.Service(independent.mushroomURL.AsDereference().String())
	if err != nil {
		return fmt.Errorf("topology.Service: %w", err)
	}

	depURLs := make(map[string]struct{})

	for _, hdep := range serviceConfig.HandlerDeps {
		for _, u := range hdep.Proxies {
			link, err := tp.GetLink(u)
			if err != nil {
				return fmt.Errorf("topology.GetLink('%s'): %w", u, err)
			}
			proxyMushroomURL, err := mushroom.New(link)
			if err != nil {
				return fmt.Errorf("mushroom.New('%s'): %w", u, err)
			}
			svcDep, err := tp.Service(proxyMushroomURL.AsDereference().String())
			if err == nil && (svcDep.IsIpc() || svcDep.IsInproc()) {
				depURLs[proxyMushroomURL.AsDereference().String()] = struct{}{}
			}
		}
		for _, u := range hdep.Extensions {
			link, err := tp.GetLink(u)
			if err != nil {
				return fmt.Errorf("topology.GetLink('%s'): %w", u, err)
			}
			proxyMushroomURL, err := mushroom.New(link)
			if err != nil {
				return fmt.Errorf("mushroom.New('%s'): %w", u, err)
			}
			svcDep, err := tp.Service(proxyMushroomURL.AsDereference().String())
			if err == nil && (svcDep.IsIpc() || svcDep.IsInproc()) {
				depURLs[proxyMushroomURL.AsDereference().String()] = struct{}{}
			}
		}
	}
	for _, variant := range serviceConfig.Handlers {
		h, ok := variant.AsIndependentHandler()
		if !ok {
			continue
		}
		for _, cdep := range h.CommandDeps {
			for _, u := range cdep.Proxies {
				link, err := tp.GetLink(u)
				if err != nil {
					return fmt.Errorf("topology.GetLink('%s'): %w", u, err)
				}
				proxyMushroomURL, err := mushroom.New(link)
				if err != nil {
					return fmt.Errorf("mushroom.New('%s'): %w", u, err)
				}
				svcDep, err := tp.Service(proxyMushroomURL.AsDereference().String())
				if err == nil && (svcDep.IsIpc() || svcDep.IsInproc()) {
					depURLs[proxyMushroomURL.AsDereference().String()] = struct{}{}
				}
			}
			for _, u := range cdep.Extensions {
				link, err := tp.GetLink(u)
				if err != nil {
					return fmt.Errorf("topology.GetLink('%s'): %w", u, err)
				}
				proxyMushroomURL, err := mushroom.New(link)
				if err != nil {
					return fmt.Errorf("mushroom.New('%s'): %w", u, err)
				}
				svcDep, err := tp.Service(proxyMushroomURL.AsDereference().String())
				if err == nil && (svcDep.IsIpc() || svcDep.IsInproc()) {
					depURLs[proxyMushroomURL.AsDereference().String()] = struct{}{}
				}
			}
		}
	}

	if len(depURLs) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(depURLs))
	for url := range depURLs {
		wg.Add(1)
		go func(depURL string) {
			defer wg.Done()
			running, runErr := independent.manager.IsServiceRunning(depURL, 10)
			if runErr != nil {
				errCh <- fmt.Errorf("service %q: %w", depURL, runErr)
				return
			}
			if running {
				return
			}
			errCh <- fmt.Errorf("service %q did not become running after %d attempts", depURL, 10)
		}(url)
	}
	wg.Wait()
	close(errCh)

	for e := range errCh {
		return e
	}
	return nil
}

func (independent *Extension) startIpcService(url string, startedRefs map[string]struct{}) error {
	if url == "" {
		return fmt.Errorf("dep mushroom url is empty")
	}
	mushroomURL, err := mushroom.New(url)
	if err != nil {
		return fmt.Errorf("mushroom.New('%s'): %w", url, err)
	}

	depService, err := independent.topology().Service(mushroomURL.AsDereference().String())
	if err != nil {
		return err
	}
	if _, done := startedRefs[depService.Name]; done {
		return nil
	}
	startedRefs[depService.Name] = struct{}{}

	if err := independent.startIpcServicesFor(depService, startedRefs); err != nil {
		return fmt.Errorf("service %q ipc deps: %w", depService.Name, err)
	}
	if !depService.IsIpc() {
		return nil
	}
	if len(depService.StartCommand) == 0 {
		return fmt.Errorf("service '%s' has no start command given", depService.Name)
	}

	running, err := independent.manager.IsServiceRunning(depService.Name)
	if err != nil {
		return fmt.Errorf("manager.IsServiceRunning('%s'): %w", depService.Name, err)
	}
	if running {
		return nil
	}
	if _, err := independent.manager.StartService(depService.Name); err != nil {
		return fmt.Errorf("manager.StartService('%s'): %w", depService.Name, err)
	}
	return nil
}

func (independent *Extension) allowServiceManager() error {
	tp := independent.topology()
	serviceConfig, err := tp.Service(independent.mushroomURL.AsDereference().String())
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", independent.mushroomURL, err)
	}

	managerLink, err := mushroom.New(independent.mushroomURL.New(config.ServiceManagerCategory).String())
	if err != nil {
		return fmt.Errorf("mushroom.New('%s'): %w", independent.mushroomURL.New(config.ServiceManagerCategory).String(), err)
	}
	publicKey := independent.manager.PublicKey()

	if serviceConfig.Parameters == nil {
		serviceConfig.Parameters = datatype.New()
	}
	if existing, _ := serviceConfig.Parameters[manager.ManagerPublicKeyParam].(string); existing != publicKey {
		serviceConfig.Parameters[manager.ManagerPublicKeyParam] = publicKey
		if err := tp.SetService(serviceConfig); err != nil {
			return fmt.Errorf("topology.SetService('%s') store public key: %w", independent.mushroomURL, err)
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
		svcMushroomURL, err := mushroom.New(svcURL)
		if err != nil {
			return fmt.Errorf("mushroom.New('%s'): %w", svcURL, err)
		}
		depService, err := tp.Service(svcMushroomURL.AsDereference().String())
		if err != nil {
			return fmt.Errorf("topology.Service('%s'): %w", svcURL, err)
		}
		if !depServiceNeedsManagerAllow(depService.Parameters, managerLink, independent.mushroomURL.ResourcePublicKey()) {
			continue
		}
		setDepServiceManagerAllow(&depService, config.ServiceManagerCategory, managerLink, independent.mushroomURL.ResourcePublicKey())
		if err := tp.SetService(depService); err != nil {
			return fmt.Errorf("topology.SetService('%s'): %w", depService.Name, err)
		}
	}

	return nil
}

func (independent *Extension) addAllowedManagerClients(parameters datatype.KeyValue) error {
	if parameters == nil {
		if independent.logger != nil {
			independent.logger.Warn("no allowed keys: parameters not set, no one can access this service", "service", independent.mushroomURL)
		}
		return nil
	}

	allowed, ok := parameters["allowed"]
	if !ok {
		if independent.logger != nil {
			independent.logger.Warn("no allowed keys: 'allowed' parameter missing, no one can access this service", "service", independent.mushroomURL)
		}
		return nil
	}

	categoryMap, ok := allowed.(map[string]interface{})
	if !ok {
		if independent.logger != nil {
			independent.logger.Warn("no allowed keys: 'allowed' parameter has unexpected type", "service", independent.mushroomURL)
		}
		return nil
	}

	managerEntry, ok := categoryMap[config.ServiceManagerCategory]
	if !ok {
		if independent.logger != nil {
			independent.logger.Warn("no allowed keys: service manager category not found in allowed", "service", independent.mushroomURL, "category", config.ServiceManagerCategory)
		}
		return nil
	}

	entryMap, ok := managerEntry.(map[string]interface{})
	if !ok {
		if independent.logger != nil {
			independent.logger.Warn("no allowed keys: manager allowed entry has unexpected type", "service", independent.mushroomURL)
		}
		return nil
	}

	for link, pubKeyVal := range entryMap {
		pubKey, ok := pubKeyVal.(string)
		if !ok || pubKey == "" {
			continue
		}
		independent.manager.Allow(pubKey)
		fmt.Printf("The %s allowed to access: %s\n", independent.mushroomURL, link)
	}

	return nil
}

// For every proxy in a command’s chain, figure out who it forwards to,
// write that into the proxy’s config, save it, and tell the running proxy to reload.
func (independent *Extension) syncCommandOutbounds() error {
	tp := independent.topology()
	serviceConfig, err := tp.Service(independent.mushroomURL.AsDereference().String())
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", independent.mushroomURL, err)
	}

	for handlerIndex := range serviceConfig.Handlers {
		handler, _ := serviceConfig.Handlers[handlerIndex].AsIndependentHandler()
		if handler.Category == config.ServiceManagerCategory || len(handler.CommandDeps) == 0 {
			continue
		}

		for depIndex := range handler.CommandDeps {
			dep := &handler.CommandDeps[depIndex]
			for proxyIndex := range dep.Proxies {
				proxyURL := dep.Proxies[proxyIndex]
				var outboundURL string
				var err error
				if proxyIndex+1 < len(dep.Proxies) {
					proxyMushroomURL, err := mushroom.New(dep.Proxies[proxyIndex+1])
					if err != nil {
						return fmt.Errorf("mushroom.New('%s'): %w", dep.Proxies[proxyIndex+1], err)
					}
					outboundURL, err = tp.GetFacade(proxyMushroomURL.AsDereference().String(), dep.Name)
				} else {
					outboundURL = independent.mushroomURL.New(handler.Category).String()
				}
				if err != nil {
					return err
				}
				if err := independent.syncCommandProxyOutbound(dep.Name, proxyURL, outboundURL); err != nil {
					return fmt.Errorf("handler %q command %q proxy %q: %w", handler.Category, dep.Name, proxyURL, err)
				}
			}
		}
	}

	return nil
}

// For a handler depenency get the outbound:
// 1) If there are another handler dependency, get that service facade
// 2) If there are routes that matches the command deps, then get that outbound as secondary outbounds
// 3) If no deps then get the service itself
func (independent *Extension) handlerDepProxyOutboundTargets(handlerConfig config.Handler, proxies []string, proxyIndex int, routes []string) (string, map[string]string, error) {
	tp := independent.topology()
	if proxyIndex+1 < len(proxies) {
		proxyMushroomURL, err := mushroom.New(proxies[proxyIndex+1])
		if err != nil {
			return "", nil, fmt.Errorf("mushroom.New('%s'): %w", proxies[proxyIndex+1], err)
		}
		outboundURL, err := tp.GetFacade(proxyMushroomURL.AsDereference().String())
		return outboundURL, nil, err
	}

	commandOutbounds := make(map[string]string)
	for _, route := range routes {
		commandDep, ok := commandDepByName(handlerConfig, route)
		if !ok || len(commandDep.Proxies) == 0 {
			continue
		}
		proxyMushroomURL, err := mushroom.New(commandDep.Proxies[0])
		if err != nil {
			return "", nil, fmt.Errorf("mushroom.New('%s'): %w", commandDep.Proxies[0], err)
		}
		outboundURL, err := tp.GetFacade(proxyMushroomURL.AsDereference().String(), route)
		if err != nil {
			return "", nil, fmt.Errorf("command %q first proxy: %w", route, err)
		}
		commandOutbounds[route] = outboundURL
	}

	handler, ok := handlerConfig.AsIndependentHandler()
	if !ok {
		return "", nil, fmt.Errorf("handler is not an independent handler")
	}
	outboundURL := independent.mushroomURL.New(handler.Category).String()
	return outboundURL, commandOutbounds, nil
}

func (independent *Extension) syncHandlerDepProxyOutbounds(routes []string, proxyHandlerUrl string, outboundURL string, commandOutbounds map[string]string) error {
	proxyMushroomURL, err := mushroom.New(proxyHandlerUrl)
	if err != nil {
		return fmt.Errorf("mushroom.New('%s'): %w", proxyHandlerUrl, err)
	}
	handler, err := independent.topology().Handler(proxyMushroomURL.AsDereference().String())
	if err != nil {
		return err
	}
	proxyHandler, ok := handler.AsProxyHandler()
	if !ok {
		return fmt.Errorf("dep %q is not a proxy handler", proxyHandlerUrl)
	}
	proxyHandler, ok = normalizeProxyHandlerOutbounds(proxyHandler).AsProxyHandler()
	if !ok {
		return fmt.Errorf("dep %q is not a proxy handler", proxyHandlerUrl)
	}
	updated := false
	if !stringSlicesEqual(proxyHandler.Routes, routes) {
		proxyHandler.Routes = append([]string(nil), routes...)
		updated = true
	}
	outbounds := append([]string(nil), proxyHandler.Outbounds...)
	if outboundURL != "" {
		outbounds = appendUnique(outbounds, outboundURL)
	}
	for command, commandOutboundURL := range commandOutbounds {
		outbounds = appendUnique(outbounds, commandOutboundURL)
		var updatedForward bool
		proxyHandler, updatedForward = ensureProxyHandlerForward(proxyHandler, command, commandOutboundURL)
		updated = updated || updatedForward
	}
	for _, forwardURL := range proxyHandler.Forward {
		outbounds = appendUnique(outbounds, forwardURL)
	}
	if !stringSlicesEqual(proxyHandler.Outbounds, outbounds) {
		proxyHandler.Outbounds = outbounds
		updated = true
	}
	if updated {
		if err := independent.setTopologyHandler(proxyHandler, proxyHandlerUrl); err != nil {
			return err
		}
	}
	return nil
}

func (independent *Extension) setTopologyHandler(handler config.Handler, mushroomURL string) error {
	tp := independent.topology()
	proxyMushroomURL, err := mushroom.New(mushroomURL)
	if err != nil {
		return fmt.Errorf("mushroom.New('%s'): %w", mushroomURL, err)
	}
	handlerURL := proxyMushroomURL.HandlerLink().AsDereference().String()
	if err := tp.SetHandler(handler, handlerURL); err != nil {
		return fmt.Errorf("topology.SetHandler(%q): %w", handlerURL, err)
	}
	return nil
}

func (independent *Extension) syncCommandProxyOutbound(command string, proxyHandlerUrl string, outboundURL string) error {
	handler, err := independent.resolveTopologyHandler(proxyHandlerUrl)
	if err != nil {
		return err
	}
	proxyHandler, ok := handler.AsProxyHandler()
	if !ok {
		return fmt.Errorf("dep %q is not a proxy handler", proxyHandlerUrl)
	}
	proxyHandler, ok = normalizeProxyHandlerOutbounds(proxyHandler).AsProxyHandler()
	if !ok {
		return fmt.Errorf("dep %q is not a proxy handler", proxyHandlerUrl)
	}
	updated := false
	if !containsString(proxyHandler.Routes, command) {
		proxyHandler.Routes = append(proxyHandler.Routes, command)
		updated = true
	}
	updatedOutbound := proxyHandler.SetOutbound(outboundURL)
	updated = updated || updatedOutbound
	var updatedForward bool
	proxyHandler, updatedForward = ensureProxyHandlerForward(proxyHandler, command, outboundURL)
	updated = updated || updatedForward

	if updated {
		if err := independent.setTopologyHandler(proxyHandler, proxyHandlerUrl); err != nil {
			return err
		}
	}
	return nil
}

func (independent *Extension) Stop() error {
	if independent.topologyClient != nil {
		_ = independent.topologyClient.Close()
		independent.topologyClient = nil
	}
	return independent.manager.StopService(independent.mushroomURL.AsDereference().String())
}

func (independent *Extension) Wait() {
	if independent.blocker == nil {
		return
	}
	independent.blocker.Wait()
}
