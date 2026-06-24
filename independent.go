package service

import (
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/ahmetson/mushroom"
	"github.com/noPerfection/datatype"
	"github.com/noPerfection/log"
	clientSyncReplier "github.com/noPerfection/protocol/client/sync_replier"
	"github.com/noPerfection/protocol/handler/base"
	handlerConfig "github.com/noPerfection/protocol/handler/config"
	"github.com/noPerfection/protocol/handler/pair"
	"github.com/noPerfection/protocol/handler/publisher"
	"github.com/noPerfection/protocol/handler/replier"
	"github.com/noPerfection/protocol/handler/sync_replier"
	"github.com/noPerfection/protocol/handler/worker"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service/handlers"
	"github.com/noPerfection/service/manager"
	"github.com/noPerfection/service/package_url"
	"github.com/noPerfection/topology"
	"github.com/noPerfection/topology/config"
)

const DefaultName = "main"
const DefaultConfigPath = "noPerfection.json"
const DefaultModuleUrl = "github.com/noPerfection/service"

var DefaultServiceManagerEndpoint = message.NewEndpoint(topology.ServiceManagerCategory, 0)

// Independent keeps all necessary parameters of the independent service.
type Independent struct {
	*handlers.Handlers
	*WithHardcodedTopology
	topologyHandler *topology.Handler // topology handles the configuration and dependencies
	topology        *topology.Client
	mushroomURL     string
	blocker         *sync.WaitGroup
	manager         *manager.Manager // manage this service from other parts
	logger          *log.Logger
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
//     service record in topology, then DefaultServiceManagerEndpoint.
//
// Examples:
//
//	// Root service "main", default config and manager from topology.
//	app, err := New("main", "noPerfection.json")
//
//	// Same service, remote manager endpoint overrides topology.
//	app, err := New("main", "noPerfection.json", message.NewEndpoint("manager", 9100))
func New(params ...any) (*Independent, error) {
	mushroomURL := DefaultName
	configPath := DefaultConfigPath
	managerEndpoint := DefaultServiceManagerEndpoint

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

	independent := &Independent{
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
func (independent *Independent) EnableLogger(enable bool) error {
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
func (independent *Independent) addDefaultServiceToTopology() error {
	serviceConfig, err := independent.topologyHandler.Service(independent.mushroomURL)
	if err == nil {
		return nil
	}

	serviceConfig = config.Service{
		Type:     config.IndependentType,
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

func (independent *Independent) addAiExtension() error {
	_, err := independent.topologyHandler.Service(AiServiceName)
	if err != nil {
		if err := independent.topologyHandler.AddService(defaultAiExtensionServiceConfig()); err != nil {
			return fmt.Errorf("topologyHandler.AddService(%q): %w", AiServiceName, err)
		}
	}

	serviceConfig, err := independent.topologyHandler.Service(independent.mushroomURL)
	if err != nil {
		return fmt.Errorf("topologyHandler.Service(%q): %w", independent.mushroomURL, err)
	}

	serviceConfig.HandlerDeps = appendHandlerExtensionDep(
		serviceConfig.HandlerDeps,
		ServiceManagerCategory,
		aiExtensionServiceLink(),
	)
	if err := independent.topologyHandler.SetService(serviceConfig, serviceParentURL(independent.mushroomURL)...); err != nil {
		return fmt.Errorf("topologyHandler.SetService(%q): %w", independent.mushroomURL, err)
	}

	return nil
}

// addDefaultHandlerToTopology adds the default handler when no handlers exist.
// Unless there are handlers set by you or others
func (independent *Independent) addDefaultHandlerToTopology() error {
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

	defaultHandler := config.IndependentHandler{
		Category: handlers.DefaultHandlerCategory,
		Endpoint: handlers.DefaultHandlerEndpoint,
		Type:     config.ReplierType,
	}
	serviceConfig.Handlers = []config.Handler{defaultHandler}
	if err := independent.topologyHandler.SetService(serviceConfig, serviceParentURL(independent.mushroomURL)...); err != nil {
		return fmt.Errorf("topologyHandler.SetService('%s'): %w", independent.mushroomURL, err)
	}

	return nil
}

// addServiceManagerToTopology stores a non-default manager handler.
// If topology already has the same manager endpoint, then do nothing.
func (independent *Independent) addServiceManagerToTopology() error {
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
	if managerConfig.Endpoint == DefaultServiceManagerEndpoint {
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

func newHandler(handlerType config.HandlerType) (base.Interface, error) {
	switch handlerType {
	case config.SyncReplierType:
		return sync_replier.New(), nil
	case config.ReplierType:
		return replier.New(), nil
	case config.PublisherType:
		return publisher.New(), nil
	case config.PairType:
		return pair.New(), nil
	case config.WorkerType:
		return worker.New(), nil
	default:
		return nil, fmt.Errorf("unsupported handler type: %s", handlerType)
	}
}

// addTopologyHandlersToHandlers adds the handlers to the handlers list.
// Except for the Service Manager category, any handler defined in the topology is
// registered in the handlers package for launching them.
func (independent *Independent) addTopologyHandlersToHandlers() error {
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
func (independent *Independent) Start() error {
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
	// AI is built in extension
	if err = independent.addAiExtension(); err != nil {
		err = fmt.Errorf("addAiExtension: %w", err)
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
	defer func() {
		_ = independent.topology.Close()
		independent.topology = nil
	}()

	if err = independent.syncCommandOutbounds(); err != nil {
		err = fmt.Errorf("syncCommandOutbounds: %w", err)
		goto errOccurred
	}
	if err = independent.syncHandlerDepOutbounds(); err != nil {
		err = fmt.Errorf("syncHandlerDepOutbounds: %w", err)
		goto errOccurred
	}
	if err = independent.validateProtocolOrders(); err != nil {
		err = fmt.Errorf("validateProtocolOrders: %w", err)
		goto errOccurred
	}
	if inprocServices, err = independent.validateInprocServiceManagers(); err != nil {
		err = fmt.Errorf("validateInprocServiceManagers: %w", err)
		goto errOccurred
	}
	if inprocServices > 0 {
		if err = independent.setupInproc(); err != nil {
			err = fmt.Errorf("setupInproc: %w", err)
			goto errOccurred
		}

	} else {
		fmt.Println("todo: inproc are 0, make sure that inproc_topology is not running at all")
	}
	if err = independent.startIpcServices(); err != nil {
		err = fmt.Errorf("startIpcServices: %w", err)
		goto errOccurred
	}

errOccurred:
	if err != nil {
		if independent.manager != nil && independent.manager.Running() {
			closeErr := independent.manager.StopService(independent.mushroomURL)
			if closeErr != nil {
				err = fmt.Errorf("%v: manager.StopService: %w", err, closeErr)
			}
		}
	}

	return err
}

func (independent *Independent) setupInproc() error {
	serviceConfig, err := independent.topology.Service(independent.mushroomURL)
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", independent.mushroomURL, err)
	}
	if serviceConfig.ModuleUrl == "" {
		return fmt.Errorf("no mushroom url for service %q", independent.mushroomURL)
	}

	exists, err := package_url.IsFileExist(serviceConfig.ModuleUrl, "inproc_topology.go")

	if err != nil || !exists {
		fmt.Println("todo: generate an extension at inproc_topology.go")
	} else {
		fmt.Println("extension exists verify it")
	}

	services, err := independent.topology.Services()
	if err != nil {
		return fmt.Errorf("topology.Services: %w", err)
	}
	for _, service := range services {
		if service.Name == serviceConfig.Name {
			continue
		}
		if !service.IsInproc() {
			continue
		}
		pkgInfo, err := package_url.New(service.ModuleUrl)
		if err != nil {
			return fmt.Errorf("package_url.New(%s): %w", service.ModuleUrl, err)
		}
		if pkgInfo.IsMain() {
			packageName := package_url.ServiceNameToPackageName(service.Name)
			moduleID := fmt.Sprintf("services/%s", packageName)
			moduleFilename := path.Join(pkgInfo.Dir(), fmt.Sprintf("services/%s/service.go", packageName))
			exists, err := pkgInfo.IsModuleExist(moduleID)
			if err != nil {
				return fmt.Errorf("package_url.IsModuleExist(%s): %w", moduleID, err)
			}
			moduleInfo := pkgInfo.NewModule(moduleID, moduleFilename)
			if !exists {
				fmt.Println("todo: using ai convert : ", pkgInfo.SourceFiles(), " to ", moduleInfo.SourceFiles(), " main module to ", moduleInfo.SourceFiles())
			}
			// Second we import it
			fmt.Println("todo: import and update the inproc_topology.go to include the service: ", moduleInfo.MushroomLink().ModuleID)
			fmt.Println("todo: find the main.go and update the module url to the new module: ", moduleInfo.MushroomLink().ModuleID)
		} else {
			fmt.Println("todo: make sure it exists")
		}
		// For now we work with one service only
		fmt.Println("todo: remove break here")
		break
	}

	return fmt.Errorf("Inproc detected, code edited instead of running, please rebuild the service")
}

func (independent *Independent) syncHandlerDepOutbounds() error {
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
func (independent *Independent) startIpcServices() error {
	serviceConfig, err := independent.topology.Service(independent.mushroomURL)
	if err != nil {
		return fmt.Errorf("topologyClient.Service('%s'): %w", independent.mushroomURL, err)
	}

	startedRefs := make(map[string]struct{})
	return independent.startIpcServicesFor(serviceConfig, startedRefs)
}

func (independent *Independent) startIpcServicesFor(serviceConfig config.Service, startedRefs map[string]struct{}) error {
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

func (independent *Independent) startIpcService(mushroomURL string, startedRefs map[string]struct{}) error {
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
	if _, err := independent.manager.StartServiceByConfig(depService); err != nil {
		return fmt.Errorf("manager.StartServiceByConfig('%s'): %w", depService.Name, err)
	}
	return nil
}

// Validate protocol orders for all services and handlers in the topology:
// tcp can forward to tcp, but not other protocols.
// ipc can forward to ipc and tcp, but not inproc protocol.
// inproc can forward to inproc only, but not ipc or tcp protocol.
func (independent *Independent) validateProtocolOrders() error {
	serviceConfig, err := independent.topologyService(independent.mushroomURL)
	if err != nil {
		return fmt.Errorf("service %q: %w", independent.mushroomURL, err)
	}

	return independent.validateProtocolOrdersFor(serviceConfig)
}

func (independent *Independent) validateProtocolOrdersFor(serviceConfig config.Service) error {
	if serviceConfig.Type == config.ProxyType {
		for _, variant := range serviceConfig.Handlers {
			proxyHandler, _ := variant.AsProxyHandler()
			if len(proxyHandler.Outbounds) == 0 {
				continue
			}

			for _, outboundURL := range proxyHandler.Outbounds {
				outboundService, outboundHandler, err := independent.serviceAndHandlerFromURL(outboundURL)
				if err != nil {
					return fmt.Errorf("proxy %q handler %q outbound %q: %w", serviceConfig.Name, proxyHandler.Category, outboundURL, err)
				}
				if err := validateProtocolOrder(serviceConfig, variant, outboundService, outboundHandler); err != nil {
					return fmt.Errorf("proxy %q handler %q outbound %q: %w", serviceConfig.Name, proxyHandler.Category, outboundURL, err)
				}
			}
		}
	}

	for _, dep := range serviceConfig.HandlerDeps {
		for _, proxyURL := range dep.Proxies {
			proxyService, _, err := independent.serviceAndHandlerFromURL(proxyURL)
			if err != nil {
				return fmt.Errorf("handler dep %q proxy %q: %w", dep.Name, proxyURL, err)
			}
			if err := independent.validateProtocolOrdersFor(proxyService); err != nil {
				return fmt.Errorf("handler dep %q proxy %q: %w", dep.Name, proxyURL, err)
			}
		}

		for _, extensionURL := range dep.Extensions {
			extensionService, _, err := independent.serviceAndHandlerFromURL(extensionURL)
			if err != nil {
				return fmt.Errorf("handler dep %q extension %q: %w", dep.Name, extensionURL, err)
			}
			if err := independent.validateProtocolOrdersFor(extensionService); err != nil {
				return fmt.Errorf("handler dep %q extension %q: %w", dep.Name, extensionURL, err)
			}
		}
	}

	for _, variant := range serviceConfig.Handlers {
		handler, ok := variant.AsIndependentHandler()
		if !ok {
			continue
		}
		if handler.Category == topology.ServiceManagerCategory || len(handler.CommandDeps) == 0 {
			continue
		}

		for _, dep := range handler.CommandDeps {
			for _, proxyURL := range dep.Proxies {
				proxyService, _, err := independent.serviceAndHandlerFromURL(proxyURL)
				if err != nil {
					return fmt.Errorf("handler %q command %q proxy %q: %w", handler.Category, dep.Name, proxyURL, err)
				}
				if err := independent.validateProtocolOrdersFor(proxyService); err != nil {
					return fmt.Errorf("handler %q command %q proxy %q: %w", handler.Category, dep.Name, proxyURL, err)
				}
			}
			for _, extensionURL := range dep.Extensions {
				extensionService, _, err := independent.serviceAndHandlerFromURL(extensionURL)
				if err != nil {
					return fmt.Errorf("handler %q command %q extension %q: %w", handler.Category, dep.Name, extensionURL, err)
				}
				if err := independent.validateProtocolOrdersFor(extensionService); err != nil {
					return fmt.Errorf("handler %q command %q extension %q: %w", handler.Category, dep.Name, extensionURL, err)
				}
			}
		}
	}

	return nil
}

func (independent *Independent) topologyService(mushroomURL string) (config.Service, error) {
	mushroomURL = dereferenceMushroomURL(mushroomURL)
	if independent.topology != nil {
		return independent.topology.Service(mushroomURL)
	}
	if independent.topologyHandler != nil {
		return independent.topologyHandler.Service(mushroomURL)
	}
	return config.Service{}, fmt.Errorf("topology is nil")
}

func dereferenceMushroomURL(url string) string {
	if url == "" {
		return url
	}
	var soil mushroom.Soil
	hypha, err := soil.Hypha(url)
	if err != nil || !hypha.URL {
		return url
	}
	return hypha.AsDereference().String()
}

func isServiceOnlyMushroomURL(mushroomURL string) bool {
	return strings.Contains(mushroomURL, "services[name:") && !strings.Contains(mushroomURL, ".handlers[")
}

func handlerCategoryFromMushroomURL(mushroomURL string) string {
	var soil mushroom.Soil
	hypha, err := soil.Hypha(mushroomURL)
	if err != nil || !hypha.URL {
		return topology.DefaultCategory
	}
	if category := hypha.AdditionalProps["category"]; category != "" {
		return category
	}
	return topology.DefaultCategory
}

func normalizeProxyHandlerOutbounds(handler config.Handler) config.Handler {
	proxyHandler, ok := handler.AsProxyHandler()
	if !ok || proxyHandler.Outbounds != nil {
		return handler
	}
	proxyHandler.Outbounds = []string{}
	return proxyHandler
}

func (independent *Independent) resolveTopologyHandler(mushroomURL string) (config.Handler, error) {
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

func (independent *Independent) serviceAndHandlerFromURL(mushroomURL string) (config.Service, config.Handler, error) {
	if mushroomURL == "" {
		return config.Service{}, nil, fmt.Errorf("dep mushroom url is empty")
	}
	handler, err := independent.resolveTopologyHandler(mushroomURL)
	if err != nil {
		return config.Service{}, nil, fmt.Errorf("topology.Handler(%q): %w", mushroomURL, err)
	}
	service, err := independent.topologyService(mushroomURL)
	if err != nil {
		return config.Service{}, nil, err
	}
	return service, handler, nil
}

func (independent *Independent) GetHandlerLink(handlerCategory string) (string, error) {
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

func (independent *Independent) GetServiceFacade(mushroomURL string, command ...string) (string, error) {
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

func validateProtocolOrder(callerService config.Service, caller config.Handler, outboundService config.Service, outbound config.Handler) error {
	callerHandler, ok := caller.AsIndependentHandler()
	if !ok {
		return fmt.Errorf("caller handler is not an independent handler")
	}
	outboundHandler, ok := outbound.AsIndependentHandler()
	if !ok {
		return fmt.Errorf("outbound handler is not an independent handler")
	}

	callerInproc, err := callerService.IsInprocHandler(callerHandler.Category)
	if err != nil {
		return err
	}
	if callerInproc {
		return nil
	}

	outboundInproc, err := outboundService.IsInprocHandler(outboundHandler.Category)
	if err != nil {
		return err
	}
	callerProtocol := "tcp"
	if callerHandler.Endpoint.IsIpc() {
		callerProtocol = "ipc"
	}
	outboundProtocol := "tcp"
	if outboundInproc {
		outboundProtocol = "inproc"
	} else if outboundHandler.Endpoint.IsIpc() {
		outboundProtocol = "ipc"
	}

	if callerProtocol == "ipc" && !outboundInproc {
		return nil
	}
	if callerProtocol == "tcp" && outboundProtocol == "tcp" {
		return nil
	}
	return fmt.Errorf("can not access from %s to %s", callerProtocol, outboundProtocol)
}

// If service is inproc, it must have an inproc manager.
func (independent *Independent) validateInprocServiceManagers() (int, error) {
	inprocServices := 0
	serviceConfig, err := independent.topology.Service(independent.mushroomURL)
	if err != nil {
		return 0, err
	}
	if err := independent.validateInprocServiceManagersFor(serviceConfig, &inprocServices); err != nil {
		return 0, err
	}
	// Its still incremented, but we don't count it
	if serviceConfig.IsInproc() {
		inprocServices--
	}
	return inprocServices, nil
}

func (independent *Independent) validateInprocServiceManagersFor(serviceConfig config.Service, inprocServices *int) error {
	if serviceConfig.IsInproc() {
		endpoint, err := serviceManagerEndpoint(serviceConfig)
		if err != nil {
			return err
		}
		if !endpoint.IsInproc() {
			return fmt.Errorf("service %q is inproc but manager endpoint %q is not inproc", serviceConfig.Name, endpoint.ClientUrl())
		}
		(*inprocServices)++
	}

	for _, dep := range serviceConfig.HandlerDeps {
		for _, link := range dep.Proxies {
			depService, err := independent.topologyService(link)
			if err != nil {
				return fmt.Errorf("handler dep %q proxy %q: %w", dep.Name, link, err)
			}
			if err := independent.validateInprocServiceManagersFor(depService, inprocServices); err != nil {
				return fmt.Errorf("handler dep %q proxy %q: %w", dep.Name, link, err)
			}
		}
		for _, link := range dep.Extensions {
			depService, err := independent.topologyService(link)
			if err != nil {
				return fmt.Errorf("handler dep %q extension %q: %w", dep.Name, link, err)
			}
			if err := independent.validateInprocServiceManagersFor(depService, inprocServices); err != nil {
				return fmt.Errorf("handler dep %q extension %q: %w", dep.Name, link, err)
			}
		}
	}

	for _, variant := range serviceConfig.Handlers {
		handler, ok := variant.AsIndependentHandler()
		if !ok {
			continue
		}
		for _, dep := range handler.CommandDeps {
			for _, link := range dep.Proxies {
				depService, err := independent.topologyService(link)
				if err != nil {
					return fmt.Errorf("handler %q command %q proxy %q: %w", handler.Category, dep.Name, link, err)
				}
				if err := independent.validateInprocServiceManagersFor(depService, inprocServices); err != nil {
					return fmt.Errorf("handler %q command %q proxy %q: %w", handler.Category, dep.Name, link, err)
				}
			}
			for _, link := range dep.Extensions {
				depService, err := independent.topologyService(link)
				if err != nil {
					return fmt.Errorf("handler %q command %q extension %q: %w", handler.Category, dep.Name, link, err)
				}
				if err := independent.validateInprocServiceManagersFor(depService, inprocServices); err != nil {
					return fmt.Errorf("handler %q command %q extension %q: %w", handler.Category, dep.Name, link, err)
				}
			}
		}
	}

	return nil
}

func serviceManagerEndpoint(serviceConfig config.Service) (message.Endpoint, error) {
	managerHandler, err := serviceConfig.HandlerByCategory(topology.ServiceManagerCategory)
	if err != nil {
		if serviceConfig.Type == config.ProxyType {
			return manager.DefaultProxyManagerEndpoint(serviceConfig.Name), nil
		}
		return DefaultServiceManagerEndpoint, nil
	}
	handler, ok := managerHandler.AsIndependentHandler()
	if !ok {
		return message.Endpoint{}, fmt.Errorf("service %q manager handler is not an independent handler", serviceConfig.Name)
	}
	return handler.Endpoint, nil
}

// For every proxy in a command’s chain, figure out who it forwards to,
// write that into the proxy’s config, save it, and tell the running proxy to reload.
func (independent *Independent) syncCommandOutbounds() error {
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
func (independent *Independent) handlerDepProxyOutboundTargets(handlerConfig config.Handler, proxies []string, proxyIndex int, routes []string) (string, map[string]string, error) {
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

func (independent *Independent) syncHandlerDepProxyOutbounds(routes []string, proxyHandlerUrl string, outboundURL string, commandOutbounds map[string]string) error {
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

func commandDepByName(handlerConfig config.Handler, command string) (config.DepService, bool) {
	handler, ok := handlerConfig.AsIndependentHandler()
	if !ok {
		return config.DepService{}, false
	}
	for _, dep := range handler.CommandDeps {
		if dep.Name == command {
			return dep, true
		}
	}
	return config.DepService{}, false
}

func (independent *Independent) setTopologyHandler(handler config.Handler, mushroomURL string) error {
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

func (independent *Independent) syncCommandProxyOutbound(command string, proxyHandlerUrl string, outboundURL string) error {
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

func ensureProxyHandlerForward(proxyConfig config.ProxyHandler, command string, outboundURL string) (config.ProxyHandler, bool) {
	if proxyConfig.Forward == nil {
		proxyConfig.Forward = make(map[string]string)
	}
	if proxyConfig.Forward[command] == outboundURL {
		return proxyConfig, false
	}
	proxyConfig.Forward[command] = outboundURL
	proxyConfig.SetOutbound(outboundURL)
	return proxyConfig, true
}

func stringSlicesEqual(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func newProxyManagerClient(proxyService config.Service) (*clientSyncReplier.Client, error) {
	endpoint := manager.DefaultProxyManagerEndpoint(proxyService.Name)
	if managerHandler, err := proxyService.HandlerByCategory(topology.ServiceManagerCategory); err == nil {
		handler, ok := managerHandler.AsIndependentHandler()
		if ok {
			endpoint = handler.Endpoint
		}
	}
	client, err := clientSyncReplier.NewClient(endpoint.Id, endpoint.Port)
	if err != nil {
		return nil, err
	}
	client.Timeout(time.Second)
	client.Attempt(1)
	return client, nil
}

func reloadProxy(proxyService config.Service, proxyConfig config.ProxyHandler, updated bool) error {
	return nil // TODO: implement hot reload later not from the outbound, but by handshake
	if !updated {
		return nil
	}
	proxyManagerClient, err := newProxyManagerClient(proxyService)
	if err != nil {
		return err
	}
	defer proxyManagerClient.Close()

	exists, err := proxyHandlerExists(proxyManagerClient, proxyService.Name, proxyConfig.Category)
	if err != nil {
		return nil
	}
	if !exists {
		if err := setProxyHandler(proxyManagerClient, proxyService.Name, proxyConfig); err != nil {
			return err
		}
		return startProxyHandler(proxyManagerClient, proxyService.Name, proxyConfig.Category)
	}

	running, err := proxyHandlerRunning(proxyManagerClient, proxyService.Name, proxyConfig.Category)
	if err != nil {
		return err
	}
	if running {
		if err := stopProxyHandler(proxyManagerClient, proxyService.Name, proxyConfig.Category); err != nil {
			return err
		}
	}
	if err := setProxyHandler(proxyManagerClient, proxyService.Name, proxyConfig); err != nil {
		return err
	}
	return startProxyHandler(proxyManagerClient, proxyService.Name, proxyConfig.Category)
}

func proxyHandlerExists(client *clientSyncReplier.Client, serviceName string, category string) (bool, error) {
	reply, err := proxyManagerRequest(client, handlers.IsProxyHandlerExistCommand, datatype.New().Set("service", serviceName).Set("category", category))
	if err != nil {
		return false, err
	}
	return reply.ReplyParameters().BoolValue("exists")
}

func proxyHandlerRunning(client *clientSyncReplier.Client, serviceName string, category string) (bool, error) {
	reply, err := proxyManagerRequest(client, handlers.IsProxyHandlerRunningCommand, datatype.New().Set("service", serviceName).Set("category", category))
	if err != nil {
		return false, err
	}
	return reply.ReplyParameters().BoolValue("running")
}

func setProxyHandler(client *clientSyncReplier.Client, serviceName string, proxyConfig config.ProxyHandler) error {
	configParams, err := datatype.NewFromInterface(proxyConfig)
	if err != nil {
		return fmt.Errorf("datatype.NewFromInterface: %w", err)
	}
	_, err = proxyManagerRequest(client, handlers.SetProxyHandlerCommand, datatype.New().Set("service", serviceName).Set("config", configParams))
	return err
}

func startProxyHandler(client *clientSyncReplier.Client, serviceName string, category string) error {
	_, err := proxyManagerRequest(client, handlers.StartProxyHandlerCommand, datatype.New().Set("service", serviceName).Set("category", category))
	return err
}

func stopProxyHandler(client *clientSyncReplier.Client, serviceName string, category string) error {
	_, err := proxyManagerRequest(client, handlers.StopProxyHandlerCommand, datatype.New().Set("service", serviceName).Set("category", category))
	return err
}

func proxyManagerRequest(client *clientSyncReplier.Client, command string, params datatype.KeyValue) (message.ReplyInterface, error) {
	reply, err := client.Request(&message.Request{
		Command:    command,
		Parameters: params,
	})
	if err != nil {
		return nil, fmt.Errorf("proxy manager request %q: %w", command, err)
	}
	if !reply.IsOK() {
		return nil, fmt.Errorf("proxy manager request %q: %s", command, reply.ErrorMessage())
	}
	return reply, nil
}

func appendUnique(values []string, value string) []string {
	if containsString(values, value) {
		return values
	}
	return append(values, value)
}

func containsString(values []string, value string) bool {
	for _, existing := range values {
		if existing == value {
			return true
		}
	}
	return false
}

func (independent *Independent) Stop() error {
	return independent.manager.StopService(independent.mushroomURL)
}

func (independent *Independent) Wait() {
	if independent.blocker == nil {
		return
	}
	independent.blocker.Wait()
}
