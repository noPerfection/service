package service

import (
	"errors"
	"fmt"
	"sync"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/log"
	"github.com/noPerfection/protocol/handler/npac"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service/handlers"
	"github.com/noPerfection/service/manager"
	"github.com/noPerfection/service/mushroom"
	"github.com/noPerfection/service/package_url"
	"github.com/noPerfection/topology"
	"github.com/noPerfection/topology/config"
)

// Extension keeps all necessary parameters of the ext service.
type Extension struct {
	*handlers.Setup
	*WithHardcodedTopology
	*TopologyConnection
	rawMushroomURL string
	mushroomURL    mushroom.TopologyURL
	blocker        *sync.WaitGroup
	manager        *manager.Manager // manage this service from other parts
	logger         *log.Logger
	ipcStarted     map[string]struct{}
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

	ext := &Extension{
		Setup:                 handlers.NewSetup(),
		WithHardcodedTopology: NewHardcodedTopologies(mushroomURL),
		TopologyConnection:    newTopologyConnection(),
		rawMushroomURL:        mushroomURL,
		logger:                nil,
	}

	return ext, nil
}

// EnableLogger toggles the optional service logger.
func (ext *Extension) EnableLogger(enable bool) error {
	if !enable {
		if err := ext.Setup.SetLogger(nil); err != nil {
			return fmt.Errorf("handlers.SetLogger: %w", err)
		}
		if ext.manager != nil {
			if err := ext.manager.SetLogger(nil); err != nil {
				return fmt.Errorf("manager.SetLogger: %w", err)
			}
		}
		ext.logger = nil
		return nil
	}

	logger, err := log.New(ext.rawMushroomURL, true)
	if err != nil {
		return fmt.Errorf("log.New(%s): %w", ext.rawMushroomURL, err)
	}
	if err := ext.Setup.SetLogger(logger); err != nil {
		return fmt.Errorf("handlers.SetLogger: %w", err)
	}

	if ext.manager != nil {
		if err := ext.manager.SetLogger(logger); err != nil {
			return fmt.Errorf("manager.SetLogger: %w", err)
		}
	}

	ext.logger = logger
	return nil
}

// addDefaultServiceToTopology adds the default service config
// if no config was given for this service.
func (ext *Extension) addDefaultServiceToTopology() error {
	tp := ext.topology()
	serviceConfig, err := tp.Service(ext.mushroomURL.AsDereference().String())
	if err == nil {
		return nil
	}

	serviceConfig = config.Service{
		Type:     config.ExtensionType,
		Name:     ext.rawMushroomURL,
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
		return fmt.Errorf("topology.AddService('%s'): %w", ext.mushroomURL, err)
	}

	return nil
}

// addDefaultHandlerToTopology adds the default handler when no handlers exist.
// Unless there are handlers set by you or others
func (ext *Extension) addDefaultHandlerToTopology() error {
	tp := ext.topology()
	serviceConfig, err := tp.Service(ext.mushroomURL.AsDereference().String())
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", ext.mushroomURL, err)
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
		return fmt.Errorf("topology.SetService('%s'): %w", ext.mushroomURL, err)
	}

	return nil
}

// ensureServiceManager creates the service manager from topology configuration.
// When the service record has a manager handler, that endpoint is used;
// otherwise manager.DefaultExtensionManagerEndpoint is used.
func (ext *Extension) ensureServiceManager() error {
	tp := ext.topology()
	serviceConfig, err := tp.Service(ext.mushroomURL.AsDereference().String())
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", ext.mushroomURL, err)
	}

	managerEndpoint := manager.DefaultExtensionManagerEndpoint(ext.mushroomURL.AsDereference().String())
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

	m, err := manager.New(ext.mushroomURL, managerEndpoint, secretKey)
	if err != nil {
		return fmt.Errorf("manager.New: %w", err)
	}
	ext.manager = m
	if err := ext.manager.SetLogger(ext.logger); err != nil {
		return fmt.Errorf("manager.SetLogger: %w", err)
	}

	if err := ext.addAllowedManagerClients(serviceConfig.Parameters); err != nil {
		return fmt.Errorf("addAllowedManagerClients: %w", err)
	}

	return nil
}

// addTopologyHandlersToHandlers adds the handlers to the handlers list.
// Except for the Service Manager category, any handler defined in the topology is
// registered in the handlers package for launching them.
func (ext *Extension) addTopologyHandlersToHandlers() error {
	tp := ext.topology()
	serviceConfig, err := tp.Service(ext.mushroomURL.AsDereference().String())
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", ext.mushroomURL, err)
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
		if err := ext.Setup.SetHandler(configured.Category, handler); err != nil {
			return fmt.Errorf("handlers.SetHandler('%s'): %w", configured.Category, err)
		}
	}

	return nil
}

// Start the service.
//
// Requires at least one handler.
func (ext *Extension) Start() error {
	var err error
	var inprocServices int
	var topologySnapshot string
	var serviceLink string
	var tp topology.TopologyInterface
	var npacAnyContextPushed bool
	if err = npac.New().Start(); err != nil {
		err = fmt.Errorf("npac.Start: %w", err)
		goto errOccurred
	}
	if err = ext.TopologyConnection.setupTopologyConnection(); err != nil {
		err = fmt.Errorf("setupTopologyConnection: %w", err)
		goto errOccurred
	}
	serviceLink, err = ext.topology().GetLink(ext.rawMushroomURL)
	if err != nil {
		err = fmt.Errorf("topology.GetLink('%s'): %w", ext.rawMushroomURL, err)
		goto errOccurred
	} else {
		ext.mushroomURL, err = mushroom.Parse(serviceLink)
		if err != nil {
			err = fmt.Errorf("mushroom.Parse('%s'): %w", serviceLink, err)
			goto errOccurred
		}
	}

	topologySnapshot, err = ext.topology().Snapshot()
	if err != nil {
		err = fmt.Errorf("topology.Snapshot: %w", err)
		goto errOccurred
	}
	if err = ext.WithHardcodedTopology.addHardcodedServicesToTopology(ext.topology()); err != nil {
		err = fmt.Errorf("addHardcodedServicesToTopology: %w", err)
		goto errOccurred
	}
	if err = ext.addDefaultServiceToTopology(); err != nil {
		err = fmt.Errorf("addDefaultServiceToTopology: %w", err)
		goto errOccurred
	}
	if err = ext.WithHardcodedTopology.addHardcodedHandlersToTopology(ext.topology()); err != nil {
		err = fmt.Errorf("addHardcodedHandlersToTopology: %w", err)
		goto errOccurred
	}
	if err = ext.addDefaultHandlerToTopology(); err != nil {
		err = fmt.Errorf("addDefaultHandlerToTopology: %w", err)
		goto errOccurred
	}

	if err = ext.WithHardcodedTopology.addHardcodedHandlerDepsToTopology(ext.topology()); err != nil {
		err = fmt.Errorf("addHardcodedHandlerDepsToTopology: %w", err)
		goto errOccurred
	}
	if err = ext.WithHardcodedTopology.addHardcodedServiceParamsToTopology(ext.topology()); err != nil {
		err = fmt.Errorf("addHardcodedServiceParamsToTopology: %w", err)
		goto errOccurred
	}
	if err = ext.WithHardcodedTopology.addHardcodedEndpointsToTopology(ext.topology()); err != nil {
		err = fmt.Errorf("addHardcodedEndpointsToTopology: %w", err)
		goto errOccurred
	}

	if err = ext.ensureServiceManager(); err != nil {
		err = fmt.Errorf("ensureServiceManager: %w", err)
		goto errOccurred
	}

	if err = ext.WithHardcodedTopology.addHardcodedCommandDepsToTopology(ext.topology()); err != nil {
		err = fmt.Errorf("addHardcodedCommandDepsToTopology: %w", err)
		goto errOccurred
	}

	if err = ext.addTopologyHandlersToHandlers(); err != nil {
		err = fmt.Errorf("addTopologyHandlers: %w", err)
		goto errOccurred
	}

	tp = ext.topology()
	if err = tp.ValidateProtocolOrder(ext.mushroomURL.AsDereference().String()); err != nil {
		err = fmt.Errorf("topology.ValidateProtocolOrder: %w", err)
		goto errOccurred
	}
	if err = tp.ValidateInprocServiceManagers(); err != nil {
		err = fmt.Errorf("topology.ValidateInprocServiceManagers: %w", err)
		goto errOccurred
	}
	if inprocServices, err = tp.InprocessDepNumber(ext.mushroomURL.AsDereference().String()); err != nil {
		err = fmt.Errorf("topology.InprocessDepNumber: %w", err)
		goto errOccurred
	}

	// Managers must have the public keys
	if ext.manager.PublicKey() == "" {
		err = fmt.Errorf("manager.PublicKey() is empty")
		goto errOccurred
	}

	if err = ext.allowServiceManager(); err != nil {
		err = fmt.Errorf("allowServiceManager: %w", err)
		goto errOccurred
	}

	if ext.TopologyConnection.topologyHandler != nil {
		if err = ext.TopologyConnection.topologyHandler.Start(); err != nil {
			err = fmt.Errorf("topologyHandler.Start(): %w", err)
			goto errOccurred
		}
	}
	if err = ext.TopologyConnection.ensureTopologyClient(); err != nil {
		err = fmt.Errorf("ensureTopologyClient: %w", err)
		goto errOccurred
	}
	if err = ext.Setup.Start(ext.mushroomURL); err != nil {
		err = fmt.Errorf("handlers.Start: %w", err)
		goto errOccurred
	}

	ext.blocker = &sync.WaitGroup{}
	ext.blocker.Add(1)

	ext.manager.SetSharedBlocker(&ext.blocker)
	if err = ext.manager.Start(); err != nil {
		err = fmt.Errorf("service.manager.Start: %w", err)
		goto errOccurred
	}

	// Now you can start AI extension.
	if err = ext.manager.NpacPushAnyContext(ext.Start); err != nil {
		err = fmt.Errorf("manager.NpacPushAnyContext: %w", err)
		goto errOccurred
	}
	npacAnyContextPushed = true
	defer func() {
		if npacAnyContextPushed && ext.manager != nil {
			_ = ext.manager.NpacPopAnyContext(ext.Start)
		}
	}()

	if err = ext.syncCommandOutbounds(); err != nil {
		err = fmt.Errorf("syncCommandOutbounds: %w", err)
		goto errOccurred
	}
	if err = ext.syncHandlerDepOutbounds(); err != nil {
		err = fmt.Errorf("syncHandlerDepOutbounds: %w", err)
		goto errOccurred
	}
	if inprocServices > 0 {
		fmt.Printf("todo: implement setupInproc() for extension only if its not running on main file\n")
	}
	if err = ext.startIpcServices(); err != nil {
		err = fmt.Errorf("startIpcServices: %w", err)
		goto errOccurred
	}
	if err = ext.manager.Handshake(); err != nil {
		err = fmt.Errorf("manager.Handshake: %w", err)
		goto errOccurred
	}

errOccurred:
	if err != nil {
		if topologySnapshot != "" {
			if rollbackErr := ext.topology().Rollback(topologySnapshot); rollbackErr != nil {
				err = fmt.Errorf("%w: topology.Rollback: %v", err, rollbackErr)
			}
		}
		if topologyCloseErr := ext.TopologyConnection.closeTopologyClient(); topologyCloseErr != nil {
			err = fmt.Errorf("%w: closeTopologyClient: %w", err, topologyCloseErr)
		}
		if ext.manager != nil && ext.manager.Running() {
			closeErr := ext.manager.StopService(ext.mushroomURL.AsDereference().String())
			if closeErr != nil {
				err = fmt.Errorf("%v: manager.StopService: %w", err, closeErr)
			}
		}
	}

	return err
}

func (ext *Extension) syncHandlerDepOutbounds() error {
	tp := ext.topology()
	serviceConfig, err := tp.Service(ext.mushroomURL.AsDereference().String())
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", ext.mushroomURL, err)
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
			return fmt.Errorf("handler dep %q is not an ext handler", dep.Name)
		}
		routes, err := ext.Setup.RouteCommands(dep.Name)
		if err != nil {
			return fmt.Errorf("handler dep %q route commands: %w", dep.Name, err)
		}
		if len(routes) == 0 {
			continue
		}

		for proxyIndex := range dep.Proxies {
			proxyURL := dep.Proxies[proxyIndex]
			outbound, commandOutbounds, err := ext.handlerDepProxyOutboundTargets(handler, dep.Proxies, proxyIndex, routes)
			if err != nil {
				return fmt.Errorf("handler %q proxy %q outbound: %w", dep.Name, proxyURL, err)
			}
			if err := ext.syncHandlerDepProxyOutbounds(routes, proxyURL, outbound, commandOutbounds); err != nil {
				return fmt.Errorf("handler %q proxy %q: %w", dep.Name, proxyURL, err)
			}
		}
	}

	return nil
}

// startIpcServices starts IPC services this service depends on.
func (ext *Extension) startIpcServices() error {
	tp := ext.topology()
	serviceConfig, err := tp.Service(ext.mushroomURL.AsDereference().String())
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", ext.mushroomURL, err)
	}

	startedRefs := make(map[string]struct{})
	return ext.startIpcServicesFor(serviceConfig, startedRefs)
}

func (ext *Extension) startIpcServicesFor(serviceConfig config.Service, startedRefs map[string]struct{}) error {
	tp := ext.topology()
	for _, dep := range serviceConfig.HandlerDeps {
		for _, proxy := range dep.Proxies {
			link, err := tp.GetLink(proxy)
			if err != nil {
				return fmt.Errorf("topology.GetLink('%s'): %w", proxy, err)
			}
			if err := ext.startIpcService(link, startedRefs); err != nil {
				return fmt.Errorf("handler dep %q proxy %q: %w", dep.Name, proxy, err)
			}
		}
		for _, extension := range dep.Extensions {
			link, err := tp.GetLink(extension)
			if err != nil {
				return fmt.Errorf("topology.GetLink('%s'): %w", extension, err)
			}
			if err := ext.startIpcService(link, startedRefs); err != nil {
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
				if err := ext.startIpcService(link, startedRefs); err != nil {
					return fmt.Errorf("handler %q command %q proxy %q: %w", handler.Category, dep.Name, proxy, err)
				}
			}
			for _, extension := range dep.Extensions {
				link, err := tp.GetLink(extension)
				if err != nil {
					return fmt.Errorf("topology.GetLink('%s'): %w", extension, err)
				}
				if err := ext.startIpcService(link, startedRefs); err != nil {
					return fmt.Errorf("handler %q command %q extension %q: %w", handler.Category, dep.Name, extension, err)
				}
			}
		}
	}

	return nil
}

func (ext *Extension) startIpcService(url string, startedRefs map[string]struct{}) error {
	if url == "" {
		return fmt.Errorf("dep mushroom url is empty")
	}
	mushroomURL, err := mushroom.Parse(url)
	if err != nil {
		return fmt.Errorf("mushroom.Parse('%s'): %w", url, err)
	}

	depService, err := ext.topology().Service(mushroomURL.AsDereference().String())
	if err != nil {
		return err
	}
	if _, done := startedRefs[depService.Name]; done {
		return nil
	}
	startedRefs[depService.Name] = struct{}{}

	if err := ext.startIpcServicesFor(depService, startedRefs); err != nil {
		return fmt.Errorf("service %q ipc deps: %w", depService.Name, err)
	}
	if !depService.IsIpc() {
		return nil
	}
	if len(depService.StartCommand) == 0 {
		return fmt.Errorf("service '%s' has no start command given", depService.Name)
	}

	derefURL := mushroomURL.AsDereference().String()
	running, err := ext.manager.IsServiceRunning(derefURL)
	if err != nil {
		if errors.Is(err, message.ErrAccessDenied) {
			return nil
		}
		return fmt.Errorf("manager.IsServiceRunning('%s'): %w", depService.Name, err)
	}
	if running {
		return nil
	}
	if _, err := ext.manager.StartService(depService.Name); err != nil {
		return fmt.Errorf("manager.StartService('%s'): %w", depService.Name, err)
	}
	ext.ipcStarted = markIpcStarted(ext.ipcStarted, depService.Name)
	running, err = ext.manager.IsServiceRunning(derefURL, 10)
	if err != nil {
		if errors.Is(err, message.ErrAccessDenied) {
			return nil
		}
		return fmt.Errorf("manager.IsServiceRunning('%s'): %w", depService.Name, err)
	}
	if !running {
		return fmt.Errorf("service %q did not become running after start", depService.Name)
	}
	return nil
}

func (ext *Extension) allowServiceManager() error {
	tp := ext.topology()
	serviceConfig, err := tp.Service(ext.mushroomURL.AsDereference().String())
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", ext.mushroomURL, err)
	}

	managerLink := ext.mushroomURL.New(config.ServiceManagerCategory)
	publicKey := ext.manager.PublicKey()

	if serviceConfig.Parameters == nil {
		serviceConfig.Parameters = datatype.New()
	}
	if existing, _ := serviceConfig.Parameters[manager.ManagerPublicKeyParam].(string); existing != publicKey {
		serviceConfig.Parameters[manager.ManagerPublicKeyParam] = publicKey
		if err := tp.SetService(serviceConfig); err != nil {
			return fmt.Errorf("topology.SetService('%s') store public key: %w", ext.mushroomURL, err)
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
		svcMushroomURL, err := mushroom.Parse(svcURL)
		if err != nil {
			return fmt.Errorf("mushroom.Parse('%s'): %w", svcURL, err)
		}
		depService, err := tp.Service(svcMushroomURL.AsDereference().String())
		if err != nil {
			return fmt.Errorf("topology.Service('%s'): %w", svcURL, err)
		}
		if !mushroom.IsAllowedPublicKeyMatch(&depService, managerLink, ext.mushroomURL.ResourcePublicKey()) {
			continue
		}
		mushroom.AddAllowedPublicKey(&depService, managerLink, ext.mushroomURL.ResourcePublicKey())
		if err := tp.SetService(depService); err != nil {
			return fmt.Errorf("topology.SetService('%s'): %w", depService.Name, err)
		}
	}

	return nil
}

func (ext *Extension) addAllowedManagerClients(parameters datatype.KeyValue) error {
	if parameters == nil {
		if ext.logger != nil {
			ext.logger.Warn("no allowed keys: parameters not set, no one can access this service", "service", ext.mushroomURL)
		}
		return nil
	}

	allowed, ok := parameters["allowed"]
	if !ok {
		if ext.logger != nil {
			ext.logger.Warn("no allowed keys: 'allowed' parameter missing, no one can access this service", "service", ext.mushroomURL)
		}
		return nil
	}

	categoryMap, ok := allowed.(map[string]interface{})
	if !ok {
		if ext.logger != nil {
			ext.logger.Warn("no allowed keys: 'allowed' parameter has unexpected type", "service", ext.mushroomURL)
		}
		return nil
	}

	managerEntry, ok := categoryMap[config.ServiceManagerCategory]
	if !ok {
		if ext.logger != nil {
			ext.logger.Warn("no allowed keys: service manager category not found in allowed", "service", ext.mushroomURL, "category", config.ServiceManagerCategory)
		}
		return nil
	}

	entryMap, ok := managerEntry.(map[string]interface{})
	if !ok {
		if ext.logger != nil {
			ext.logger.Warn("no allowed keys: manager allowed entry has unexpected type", "service", ext.mushroomURL)
		}
		return nil
	}

	for _, pubKeyVal := range entryMap {
		pubKey, ok := pubKeyVal.(string)
		if !ok || pubKey == "" {
			continue
		}
		ext.manager.Allow(pubKey)
	}
	if len(entryMap) > 0 {
		ext.manager.RequireWhitelist()
	}

	return nil
}

// For every proxy in a command’s chain, figure out who it forwards to,
// write that into the proxy’s config, save it, and tell the running proxy to reload.
func (ext *Extension) syncCommandOutbounds() error {
	tp := ext.topology()
	serviceConfig, err := tp.Service(ext.mushroomURL.AsDereference().String())
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", ext.mushroomURL, err)
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
					proxyMushroomURL, err := mushroom.Parse(dep.Proxies[proxyIndex+1])
					if err != nil {
						return fmt.Errorf("mushroom.Parse('%s'): %w", dep.Proxies[proxyIndex+1], err)
					}
					outboundURL, err = tp.GetFacade(proxyMushroomURL.AsDereference().String(), dep.Name)
				} else {
					outboundURL = ext.mushroomURL.New(handler.Category).String()
				}
				if err != nil {
					return err
				}
				if err := ext.syncCommandProxyOutbound(dep.Name, proxyURL, outboundURL); err != nil {
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
func (ext *Extension) handlerDepProxyOutboundTargets(handlerConfig config.Handler, proxies []string, proxyIndex int, routes []string) (string, map[string]string, error) {
	tp := ext.topology()
	if proxyIndex+1 < len(proxies) {
		proxyMushroomURL, err := mushroom.Parse(proxies[proxyIndex+1])
		if err != nil {
			return "", nil, fmt.Errorf("mushroom.Parse('%s'): %w", proxies[proxyIndex+1], err)
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
		proxyMushroomURL, err := mushroom.Parse(commandDep.Proxies[0])
		if err != nil {
			return "", nil, fmt.Errorf("mushroom.Parse('%s'): %w", commandDep.Proxies[0], err)
		}
		outboundURL, err := tp.GetFacade(proxyMushroomURL.AsDereference().String(), route)
		if err != nil {
			return "", nil, fmt.Errorf("command %q first proxy: %w", route, err)
		}
		commandOutbounds[route] = outboundURL
	}

	handler, ok := handlerConfig.AsIndependentHandler()
	if !ok {
		return "", nil, fmt.Errorf("handler is not an ext handler")
	}
	outboundURL := ext.mushroomURL.New(handler.Category).String()
	return outboundURL, commandOutbounds, nil
}

func (ext *Extension) syncHandlerDepProxyOutbounds(routes []string, proxyHandlerUrl string, outboundURL string, commandOutbounds map[string]string) error {
	proxyMushroomURL, err := mushroom.Parse(proxyHandlerUrl)
	if err != nil {
		return fmt.Errorf("mushroom.Parse('%s'): %w", proxyHandlerUrl, err)
	}
	handler, err := ext.topology().Handler(proxyMushroomURL.AsDereference().String())
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
		if err := ext.setTopologyHandler(proxyHandler, proxyHandlerUrl); err != nil {
			return err
		}
	}
	return nil
}

func (ext *Extension) setTopologyHandler(handler config.Handler, mushroomURL string) error {
	tp := ext.topology()
	proxyMushroomURL, err := mushroom.Parse(mushroomURL)
	if err != nil {
		return fmt.Errorf("mushroom.Parse('%s'): %w", mushroomURL, err)
	}
	handlerURL := proxyMushroomURL.HandlerLink().AsDereference().String()
	if err := tp.SetHandler(handler, handlerURL); err != nil {
		return fmt.Errorf("topology.SetHandler(%q): %w", handlerURL, err)
	}
	return nil
}

func (ext *Extension) syncCommandProxyOutbound(command string, proxyHandlerUrl string, outboundURL string) error {
	handler, err := ext.resolveTopologyHandler(proxyHandlerUrl)
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
		if err := ext.setTopologyHandler(proxyHandler, proxyHandlerUrl); err != nil {
			return err
		}
	}
	return nil
}

func (ext *Extension) stopIpcServices() error {
	tp := ext.topology()
	serviceConfig, err := tp.Service(ext.mushroomURL.AsDereference().String())
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", ext.mushroomURL, err)
	}

	lifecycle := &ipcLifecycle{
		topology: tp,
		manager:  ext.manager,
		started:  ext.ipcStarted,
	}
	return lifecycle.stopOwnedIpcServices(serviceConfig)
}

func (ext *Extension) Stop() error {
	if ext.manager != nil && !ext.manager.Running() {
		return nil
	}

	var stopErr error
	if err := ext.stopIpcServices(); err != nil {
		stopErr = joinStopErrors(stopErr, err)
	}
	if ext.topologyHandler != nil {
		if err := ext.topologyHandler.StopAllSpawnedProcesses(); err != nil {
			stopErr = joinStopErrors(stopErr, err)
		}
	}
	if ext.topologyClient != nil {
		_ = ext.topologyClient.Close()
		ext.topologyClient = nil
	}
	if err := ext.manager.StopService(ext.mushroomURL.AsDereference().String()); err != nil {
		stopErr = joinStopErrors(stopErr, err)
	}
	return stopErr
}

func (ext *Extension) Wait() {
	waitForShutdown(ext.blocker, ext.Stop)
}
