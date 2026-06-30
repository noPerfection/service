package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/ahmetson/mushroom"
	"github.com/noPerfection/log"
	handlerConfig "github.com/noPerfection/protocol/handler/config"
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
	topologyClient  *topology.Client
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

func (independent *Extension) topology() topology.TopologyInterface {
	if independent == nil {
		return nil
	}
	if independent.topologyClient != nil {
		return independent.topologyClient
	}
	return independent.topologyHandler
}

// NewExt returns an extension service instance.
//
// Optional parameters, in order:
//
//  1. mushroomURL — service identity in the configuration. A plain symbol is treated as the
//     service name at the root of the topology (e.g. "main" → services[name:main]). Full
//     mushroom paths are accepted but not validated yet.
//
//  2. configPath — topology JSON file for this process (default "noPerfection.json").
//
// Examples:
//
//	// Root service "main" with default config path.
//	app, err := NewExt("main", "noPerfection.json")
func NewExt(params ...any) (*Extension, error) {
	mushroomURL := DefaultName
	configPath := DefaultConfigPath

	if len(params) > 2 {
		return nil, fmt.Errorf("too many arguments, expected name and config path")
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

	independent := &Extension{
		Handlers:              handlers.NewHandlers(),
		WithHardcodedTopology: NewHardcodedTopologies(mushroomURL),
		topologyHandler:       topologyHandler,
		mushroomURL:           mushroomURL,
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
	tp := independent.topology()
	serviceConfig, err := tp.Service(independent.mushroomURL)
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

	if err := tp.AddService(serviceConfig, serviceParentURL(independent.mushroomURL)...); err != nil {
		return fmt.Errorf("topology.AddService('%s'): %w", independent.mushroomURL, err)
	}

	return nil
}

// addDefaultHandlerToTopology adds the default handler when no handlers exist.
// Unless there are handlers set by you or others
func (independent *Extension) addDefaultHandlerToTopology() error {
	tp := independent.topology()
	serviceConfig, err := tp.Service(independent.mushroomURL)
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
	if err := tp.SetService(serviceConfig, serviceParentURL(independent.mushroomURL)...); err != nil {
		return fmt.Errorf("topology.SetService('%s'): %w", independent.mushroomURL, err)
	}

	return nil
}

// ensureServiceManager creates the service manager from topology configuration.
// When the service record has a manager handler, that endpoint is used;
// otherwise manager.DefaultExtensionManagerEndpoint is used.
func (independent *Extension) ensureServiceManager() error {
	tp := independent.topology()
	serviceConfig, err := tp.Service(independent.mushroomURL)
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", independent.mushroomURL, err)
	}

	managerEndpoint := manager.DefaultExtensionManagerEndpoint(independent.mushroomURL)
	currentManager, err := serviceConfig.HandlerByCategory(topology.ServiceManagerCategory)
	if err == nil {
		handler := currentManager.(config.IndependentHandler)
		managerEndpoint = handler.Endpoint
	}

	m, err := manager.New(independent.mushroomURL, managerEndpoint)
	if err != nil {
		return fmt.Errorf("manager.New: %w", err)
	}
	independent.manager = m
	if err := independent.manager.SetLogger(independent.logger); err != nil {
		return fmt.Errorf("manager.SetLogger: %w", err)
	}

	return nil
}

// addTopologyHandlersToHandlers adds the handlers to the handlers list.
// Except for the Service Manager category, any handler defined in the topology is
// registered in the handlers package for launching them.
func (independent *Extension) addTopologyHandlersToHandlers() error {
	tp := independent.topology()
	serviceConfig, err := tp.Service(independent.mushroomURL)
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", independent.mushroomURL, err)
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
	var topologySnapshot string
	var tp topology.TopologyInterface
	if err = independent.connectTopologyClientIfRunning(); err != nil {
		err = fmt.Errorf("connectTopologyClientIfRunning: %w", err)
		goto errOccurred
	}
	topologySnapshot, err = independent.topology().Snapshot()
	if err != nil {
		err = fmt.Errorf("topology.Snapshot: %w", err)
		goto errOccurred
	}
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

	if err = independent.addHardcodedHandlerDepsToTopology(); err != nil {
		err = fmt.Errorf("addHardcodedHandlerDepsToTopology: %w", err)
		goto errOccurred
	}
	if err = independent.addHardcodedServiceParamsToTopology(); err != nil {
		err = fmt.Errorf("addHardcodedServiceParamsToTopology: %w", err)
		goto errOccurred
	}

	if err = independent.ensureServiceManager(); err != nil {
		err = fmt.Errorf("ensureServiceManager: %w", err)
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

	tp = independent.topology()
	if err = tp.ValidateProtocolOrder(independent.mushroomURL); err != nil {
		err = fmt.Errorf("topology.ValidateProtocolOrder: %w", err)
		goto errOccurred
	}
	if err = tp.ValidateInprocServiceManagers(); err != nil {
		err = fmt.Errorf("topology.ValidateInprocServiceManagers: %w", err)
		goto errOccurred
	}
	if inprocServices, err = tp.InprocessDepNumber(independent.mushroomURL); err != nil {
		err = fmt.Errorf("topology.InprocessDepNumber: %w", err)
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

	if err = independent.ensureTopologyClient(); err != nil {
		err = fmt.Errorf("ensureTopologyClient: %w", err)
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
		if topologySnapshot != "" {
			if rollbackErr := independent.topology().Rollback(topologySnapshot); rollbackErr != nil {
				err = fmt.Errorf("%w: topology.Rollback: %v", err, rollbackErr)
			}
		}
		if independent.topologyClient != nil {
			_ = independent.topologyClient.Close()
			independent.topologyClient = nil
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
	tp := independent.topology()
	serviceConfig, err := tp.Service(independent.mushroomURL)
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
	tp := independent.topology()
	serviceConfig, err := tp.Service(independent.mushroomURL)
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", independent.mushroomURL, err)
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
	depService, err := independent.topology().Service(dereferenceMushroomURL(mushroomURL))
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

func (independent *Extension) resolveTopologyHandler(mushroomURL string) (config.Handler, error) {
	mushroomURL = dereferenceMushroomURL(mushroomURL)
	tp := independent.topology()
	if isServiceOnlyMushroomURL(mushroomURL) {
		service, err := tp.Service(mushroomURL)
		if err != nil {
			return nil, fmt.Errorf("topology.Service(%q): %w", mushroomURL, err)
		}
		return service.HandlerByCategory(handlerCategoryFromMushroomURL(mushroomURL))
	}
	return tp.Handler(mushroomURL)
}

func (independent *Extension) GetHandlerLink(handlerCategory string) (string, error) {
	if handlerCategory == "" {
		return "", fmt.Errorf("handler category is empty")
	}
	tp := independent.topology()
	link, err := tp.GetLink(independent.mushroomURL)
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
	tp := independent.topology()
	url := dereferenceMushroomURL(mushroomURL)
	return tp.GetFacade(url, command...)
}

// For every proxy in a command’s chain, figure out who it forwards to,
// write that into the proxy’s config, save it, and tell the running proxy to reload.
func (independent *Extension) syncCommandOutbounds() error {
	tp := independent.topology()
	serviceConfig, err := tp.Service(independent.mushroomURL)
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", independent.mushroomURL, err)
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
	tp := independent.topology()
	url := dereferenceMushroomURL(mushroomURL)
	if err := tp.SetHandler(handler, url); err != nil {
		return fmt.Errorf("topology.SetHandler(%q): %w", mushroomURL, err)
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

func (independent *Extension) connectTopologyClientIfRunning() error {
	if independent == nil || independent.topologyClient != nil {
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
	independent.topologyClient = client
	return nil
}

func (independent *Extension) ensureTopologyClient() error {
	if independent == nil || independent.topologyClient != nil {
		return nil
	}
	client, err := topology.NewClient()
	if err != nil {
		return fmt.Errorf("topology.NewClient: %w", err)
	}
	independent.topologyClient = client
	return nil
}

func (independent *Extension) Stop() error {
	if independent.topologyClient != nil {
		_ = independent.topologyClient.Close()
		independent.topologyClient = nil
	}
	return independent.manager.StopService(independent.mushroomURL)
}

func (independent *Extension) Wait() {
	if independent.blocker == nil {
		return
	}
	independent.blocker.Wait()
}
