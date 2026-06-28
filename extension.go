package service

import (
	"fmt"
	"sync"

	"github.com/ahmetson/mushroom"
	"github.com/noPerfection/log"
	handlerConfig "github.com/noPerfection/protocol/handler/config"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service/handlers"
	"github.com/noPerfection/service/manager"
	"github.com/noPerfection/service/package_url"
	"github.com/noPerfection/topology"
	"github.com/noPerfection/topology/config"
)

// Extension keeps all necessary parameters of the independent service.
type Extension struct {
	*handlers.Handlers
	*WithHardcodedTopology
	topologyHandler *topology.Handler // topology handles the configuration and dependencies
	topology        *topology.Client
	mushroomURL     string
	blocker         *sync.WaitGroup
	manager         *manager.Manager // manage this service from other parts
	logger          *log.Logger
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

// New returns an independent service instance.
//
// Optional parameters, in order:
//
//  1. mushroomURL — service identity in the configuration. A plain symbol is treated as the
//     service name at the root of the topology (e.g. "main" → services[name:main]). Full
//     mushroom paths are accepted but not validated yet.
//
//  2. configPath — topology JSON file for this process (default "noPerfection.json").
//
//  3. managerEndpoint — highest-priority manager socket. Remote processes use this endpoint
//     to start, stop, and probe the service. When omitted, the endpoint is taken from the
//     service record in topology, then manager.DefaultExtensionManagerEndpoint.
//
// Examples:
//
//	// Root service "main", default config and manager from topology.
//	app, err := New("main", "noPerfection.json")
//
//	// Same service, remote manager endpoint overrides topology.
//	app, err := New("main", "noPerfection.json", message.NewEndpoint("manager", 9100))
func NewExt(params ...any) (*Extension, error) {
	mushroomURL := DefaultName
	configPath := DefaultConfigPath

	if len(params) > 3 {
		return nil, fmt.Errorf("too many arguments, expected name, config path, and manager endpoint")
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

	if len(params) > 1 && params[1] != nil {
		configPathArg, ok := params[1].(string)
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

	managerEndpoint := manager.DefaultExtensionManagerEndpoint(mushroomURL)

	// If user passes the manager endpoint, then use it,
	// otherwise try to get it from the topology config.
	if len(params) > 2 && params[2] != nil {
		managerEndpointArg, ok := params[2].(message.Endpoint)
		if !ok {
			return nil, fmt.Errorf("manager endpoint argument must be message.Endpoint")
		}
		managerEndpoint = managerEndpointArg
	} else {
		serviceConfig, err := topologyHandler.Service(mushroomURL)
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

	m, err := manager.New(mushroomURL, managerEndpoint)
	if err != nil {
		return nil, fmt.Errorf("manager.New: %w", err)
	}

	independent := &Extension{
		Handlers:              handlers.NewHandlers(),
		WithHardcodedTopology: NewHardcodedTopologies(mushroomURL),
		topologyHandler:       topologyHandler,
		mushroomURL:           mushroomURL,
		manager:               m,
		logger:                nil,
	}

	return independent, nil
}

// EnableLogger toggles the optional service logger.
func (independent *Extension) EnableLogger(enable bool) error {
	if !enable {
		if err := independent.Handlers.SetLogger(nil); err != nil {
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

	logger, err := log.New(independent.mushroomURL, true)
	if err != nil {
		return fmt.Errorf("log.New(%s): %w", independent.mushroomURL, err)
	}
	if err := independent.Handlers.SetLogger(logger); err != nil {
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
	serviceConfig, err := independent.topologyHandler.Service(independent.mushroomURL)
	if err == nil {
		return nil
	}

	serviceConfig = config.Service{
		Type:     config.ExtensionType,
		Name:     independent.mushroomURL,
		Handlers: []config.Handler{},
	}
	if serviceConfig.ModuleUrl == "" {
		moduleURL, err := package_url.FillDefaultModuleURL()
		if err != nil {
			return err
		}
		serviceConfig.ModuleUrl = moduleURL
	}

	if err := independent.topologyHandler.AddService(serviceConfig, serviceParentURL(independent.mushroomURL)...); err != nil {
		return fmt.Errorf("topologyHandler.AddService('%s'): %w", independent.mushroomURL, err)
	}

	return nil
}

// addDefaultHandlerToTopology adds the default handler when no handlers exist.
// Unless there are handlers set by you or others
func (independent *Extension) addDefaultHandlerToTopology() error {
	serviceConfig, err := independent.topologyHandler.Service(independent.mushroomURL)
	if err != nil {
		return fmt.Errorf("topologyHandler.Service('%s'): %w", independent.mushroomURL, err)
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
	if err := independent.topologyHandler.SetService(serviceConfig, serviceParentURL(independent.mushroomURL)...); err != nil {
		return fmt.Errorf("topologyHandler.SetService('%s'): %w", independent.mushroomURL, err)
	}

	return nil
}

// addServiceManagerToTopology stores a non-default manager handler.
// If topology already has the same manager endpoint, then do nothing.
func (independent *Extension) addServiceManagerToTopology() error {
	serviceConfig, err := independent.topologyHandler.Service(independent.mushroomURL)
	if err != nil {
		return fmt.Errorf("topologyHandler.Service('%s'): %w", independent.mushroomURL, err)
	}

	// Service manager's config in the handler config format.
	managerConfig := independent.manager.Config()
	currentManager, err := serviceConfig.HandlerByCategory(topology.ServiceManagerCategory)
	if err == nil {
		handler, ok := currentManager.AsIndependentHandler()
		if ok && handler.Endpoint == managerConfig.Endpoint {
			return nil
		}
	}
	if managerConfig.Endpoint == manager.DefaultExtensionManagerEndpoint(serviceConfig.Name) {
		return nil
	}

	// Converting from the handler config format to the topology's config format.
	managerTopologyConfig := config.IndependentHandler{
		Type:     config.HandlerType(managerConfig.Type),
		Category: managerConfig.Category,
		Endpoint: managerConfig.Endpoint,
	}

	serviceConfig.SetHandler(managerTopologyConfig, true)
	if err := independent.topologyHandler.SetService(serviceConfig, serviceParentURL(independent.mushroomURL)...); err != nil {
		return fmt.Errorf("topologyHandler.SetService('%s'): %w", independent.mushroomURL, err)
	}

	return nil
}

// addTopologyHandlersToHandlers adds the handlers to the handlers list.
// Except for the Service Manager category, any handler defined in the topology is
// registered in the handlers package for launching them.
func (independent *Extension) addTopologyHandlersToHandlers() error {
	serviceConfig, err := independent.topologyHandler.Service(independent.mushroomURL)
	if err != nil {
		return fmt.Errorf("topologyHandler.Service('%s'): %w", independent.mushroomURL, err)
	}

	for _, configuredVariant := range serviceConfig.Handlers {
		configured, ok := configuredVariant.AsIndependentHandler()
		if !ok {
			continue
		}
		if configured.Category == topology.ServiceManagerCategory {
			continue
		}

		handler, err := newHandler(configured.Type)
		if err != nil {
			return fmt.Errorf("newTopologyHandler('%s'): %w", configured.Category, err)
		}
		handler.SetConfig(handlerConfig.New(
			handlerConfig.HandlerType(configured.Type),
			configured.Endpoint.Id,
			configured.Category,
			configured.Endpoint.Port,
		))
		if err := independent.Handlers.SetHandler(configured.Category, handler); err != nil {
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
	if err = independent.addHardcodedServicesToTopology(); err != nil {
		err = fmt.Errorf("addHardcodedServicesToTopology: %w", err)
		goto errOccurred
	}
	if err = independent.addDefaultServiceToTopology(); err != nil {
		err = fmt.Errorf("lintDefaultTopology: %w", err)
		goto errOccurred
	}
	if err = independent.addHardcodedHandlersToTopology(); err != nil {
		err = fmt.Errorf("addHardcodedHandlersToTopology: %w", err)
		goto errOccurred
	}
	if err = independent.addDefaultHandlerToTopology(); err != nil {
		err = fmt.Errorf("addDefaultHandlerToTopology: %w", err)
		goto errOccurred
	}
	if err = independent.addServiceManagerToTopology(); err != nil {
		err = fmt.Errorf("lintManagerTopology: %w", err)
		goto errOccurred
	}

	if err = independent.addHardcodedHandlerDepsToTopology(); err != nil {
		err = fmt.Errorf("addHardcodedHandlerDepsToTopology: %w", err)
		goto errOccurred
	}
	if err = independent.addHardcodedServiceParamsToTopology(); err != nil {
		err = fmt.Errorf("addHardcodedServiceParamsToTopology: %w", err)
		goto errOccurred
	}
	if err = independent.addHardcodedCommandDepsToTopology(); err != nil {
		err = fmt.Errorf("addHardcodedCommandDepsToTopology: %w", err)
		goto errOccurred
	}

	if err = independent.addTopologyHandlersToHandlers(); err != nil {
		err = fmt.Errorf("addTopologyHandlers: %w", err)
		goto errOccurred
	}

	if err = independent.topologyHandler.ValidateProtocolOrder(independent.mushroomURL); err != nil {
		err = fmt.Errorf("topologyHandler.ValidateProtocolOrder: %w", err)
		goto errOccurred
	}
	if err = independent.topologyHandler.ValidateInprocServiceManagers(); err != nil {
		err = fmt.Errorf("topologyHandler.ValidateInprocServiceManagers: %w", err)
		goto errOccurred
	}
	if inprocServices, err = independent.topologyHandler.InprocessDepNumber(independent.mushroomURL); err != nil {
		err = fmt.Errorf("topologyHandler.InprocessDepNumber: %w", err)
		goto errOccurred
	}

	if err = independent.topologyHandler.Start(); err != nil {
		err = fmt.Errorf("topologyHandler.Start(): %w", err)
		goto errOccurred
	}
	if err = independent.Handlers.Start(); err != nil {
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

	independent.topology, err = topology.NewClient()
	if err != nil {
		err = fmt.Errorf("topology.NewClient: %w", err)
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

errOccurred:
	if err != nil {
		if independent.topology != nil {
			_ = independent.topology.Close()
			independent.topology = nil
		}
		if independent.manager != nil && independent.manager.Running() {
			closeErr := independent.manager.StopService(independent.mushroomURL)
			if closeErr != nil {
				err = fmt.Errorf("%v: manager.StopService: %w", err, closeErr)
			}
		}
	}

	return err
}

func (independent *Extension) syncHandlerDepOutbounds() error {
	serviceConfig, err := independent.topology.Service(independent.mushroomURL)
	if err != nil {
		return fmt.Errorf("topologyClient.Service('%s'): %w", independent.mushroomURL, err)
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
		routes, err := independent.Handlers.RouteCommands(dep.Name)
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
	serviceConfig, err := independent.topology.Service(independent.mushroomURL)
	if err != nil {
		return fmt.Errorf("topologyClient.Service('%s'): %w", independent.mushroomURL, err)
	}

	startedRefs := make(map[string]struct{})
	return independent.startIpcServicesFor(serviceConfig, startedRefs)
}

func (independent *Extension) startIpcServicesFor(serviceConfig config.Service, startedRefs map[string]struct{}) error {
	for _, dep := range serviceConfig.HandlerDeps {
		for _, proxy := range dep.Proxies {
			if err := independent.startIpcService(proxy, startedRefs); err != nil {
				return fmt.Errorf("handler dep %q proxy %q: %w", dep.Name, proxy, err)
			}
		}
		for _, extension := range dep.Extensions {
			if err := independent.startIpcService(extension, startedRefs); err != nil {
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
				if err := independent.startIpcService(proxy, startedRefs); err != nil {
					return fmt.Errorf("handler %q command %q proxy %q: %w", handler.Category, dep.Name, proxy, err)
				}
			}
			for _, extension := range dep.Extensions {
				if err := independent.startIpcService(extension, startedRefs); err != nil {
					return fmt.Errorf("handler %q command %q extension %q: %w", handler.Category, dep.Name, extension, err)
				}
			}
		}
	}

	return nil
}

func (independent *Extension) startIpcService(mushroomURL string, startedRefs map[string]struct{}) error {
	if mushroomURL == "" {
		return fmt.Errorf("dep mushroom url is empty")
	}
	depService, err := independent.topologyService(mushroomURL)
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

func (independent *Extension) topologyService(mushroomURL string) (config.Service, error) {
	mushroomURL = dereferenceMushroomURL(mushroomURL)
	if independent.topology != nil {
		return independent.topology.Service(mushroomURL)
	}
	if independent.topologyHandler != nil {
		return independent.topologyHandler.Service(mushroomURL)
	}
	return config.Service{}, fmt.Errorf("topology is nil")
}

func (independent *Extension) resolveTopologyHandler(mushroomURL string) (config.Handler, error) {
	mushroomURL = dereferenceMushroomURL(mushroomURL)
	if isServiceOnlyMushroomURL(mushroomURL) {
		service, err := independent.topologyService(mushroomURL)
		if err != nil {
			return nil, fmt.Errorf("topologyService(%q): %w", mushroomURL, err)
		}
		return service.HandlerByCategory(handlerCategoryFromMushroomURL(mushroomURL))
	}
	if independent.topology != nil {
		return independent.topology.Handler(mushroomURL)
	}
	if independent.topologyHandler != nil {
		return independent.topologyHandler.Handler(mushroomURL)
	}
	return nil, fmt.Errorf("topology is nil")
}

func (independent *Extension) GetHandlerLink(handlerCategory string) (string, error) {
	if handlerCategory == "" {
		return "", fmt.Errorf("handler category is empty")
	}
	var link string
	var err error
	if independent.topology != nil {
		link, err = independent.topology.GetLink(independent.mushroomURL)
	} else if independent.topologyHandler != nil {
		link, err = independent.topologyHandler.GetLink(independent.mushroomURL)
	} else {
		return "", fmt.Errorf("topology is nil")
	}
	if err != nil {
		return "", err
	}

	var soil mushroom.Soil
	hypha, err := soil.Hypha(link)
	if err != nil {
		return "", fmt.Errorf("soil.Hypha(%q): %w", link, err)
	}
	linkHypha := hypha.AsLink()
	if linkHypha.AdditionalProps == nil {
		linkHypha.AdditionalProps = map[string]string{}
	}
	linkHypha.AdditionalProps["category"] = handlerCategory
	return linkHypha.String(), nil
}

func (independent *Extension) GetServiceFacade(mushroomURL string, command ...string) (string, error) {
	if mushroomURL == "" {
		return "", fmt.Errorf("dep mushroom url is empty")
	}
	url := dereferenceMushroomURL(mushroomURL)
	if independent.topology != nil {
		return independent.topology.GetFacade(url, command...)
	}
	if independent.topologyHandler != nil {
		return independent.topologyHandler.GetFacade(url, command...)
	}
	return "", fmt.Errorf("topology is nil")
}

// For every proxy in a command’s chain, figure out who it forwards to,
// write that into the proxy’s config, save it, and tell the running proxy to reload.
func (independent *Extension) syncCommandOutbounds() error {
	serviceConfig, err := independent.topology.Service(independent.mushroomURL)
	if err != nil {
		return fmt.Errorf("topologyClient.Service('%s'): %w", independent.mushroomURL, err)
	}

	for handlerIndex := range serviceConfig.Handlers {
		handler, _ := serviceConfig.Handlers[handlerIndex].AsIndependentHandler()
		if handler.Category == topology.ServiceManagerCategory || len(handler.CommandDeps) == 0 {
			continue
		}

		for depIndex := range handler.CommandDeps {
			dep := &handler.CommandDeps[depIndex]
			for proxyIndex := range dep.Proxies {
				proxyURL := dep.Proxies[proxyIndex]
				var outboundURL string
				var err error
				if proxyIndex+1 < len(dep.Proxies) {
					outboundURL, err = independent.GetServiceFacade(dep.Proxies[proxyIndex+1], dep.Name)
				} else {
					outboundURL, err = independent.GetHandlerLink(handler.Category)
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
	if proxyIndex+1 < len(proxies) {
		outboundURL, err := independent.GetServiceFacade(proxies[proxyIndex+1])
		return outboundURL, nil, err
	}

	commandOutbounds := make(map[string]string)
	for _, route := range routes {
		commandDep, ok := commandDepByName(handlerConfig, route)
		if !ok || len(commandDep.Proxies) == 0 {
			continue
		}
		outboundURL, err := independent.GetServiceFacade(commandDep.Proxies[0], route)
		if err != nil {
			return "", nil, fmt.Errorf("command %q first proxy: %w", route, err)
		}
		commandOutbounds[route] = outboundURL
	}

	handler, ok := handlerConfig.AsIndependentHandler()
	if !ok {
		return "", nil, fmt.Errorf("handler is not an independent handler")
	}
	outboundURL, err := independent.GetHandlerLink(handler.Category)
	if err != nil {
		return "", nil, err
	}
	return outboundURL, commandOutbounds, nil
}

func (independent *Extension) syncHandlerDepProxyOutbounds(routes []string, proxyHandlerUrl string, outboundURL string, commandOutbounds map[string]string) error {
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
	url := dereferenceMushroomURL(mushroomURL)
	if independent.topology != nil {
		if err := independent.topology.SetHandler(handler, url); err != nil {
			return fmt.Errorf("topology.SetHandler(%q): %w", mushroomURL, err)
		}
		return nil
	}
	if independent.topologyHandler != nil {
		if err := independent.topologyHandler.SetHandler(handler, url); err != nil {
			return fmt.Errorf("topologyHandler.SetHandler(%q): %w", mushroomURL, err)
		}
		return nil
	}
	return fmt.Errorf("topology is nil")
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
	if independent.topology != nil {
		_ = independent.topology.Close()
		independent.topology = nil
	}
	return independent.manager.StopService(independent.mushroomURL)
}

func (independent *Extension) Wait() {
	if independent.blocker == nil {
		return
	}
	independent.blocker.Wait()
}
