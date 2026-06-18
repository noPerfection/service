package service

import (
	"fmt"
	"sync"
	"time"

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
	name            string
	blocker         *sync.WaitGroup
	manager         *manager.Manager // manage this service from other parts
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

	topologyHandler, err := topology.NewHandler(configPath)
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
		name:                  mushroomURL,
		manager:               m,
	}

	return independent, nil
}

// EnableLogger toggles the optional service logger.
func (independent *Independent) EnableLogger(enable bool) error {
	if !enable {
		if err := independent.Handlers.SetLogger(nil); err != nil {
			return fmt.Errorf("handlers.SetLogger: %w", err)
		}
		return nil
	}

	logger, err := log.New(independent.name, true)
	if err != nil {
		return fmt.Errorf("log.New(%s): %w", independent.name, err)
	}
	if err := independent.Handlers.SetLogger(logger); err != nil {
		return fmt.Errorf("handlers.SetLogger: %w", err)
	}

	if independent.manager != nil {
		if err := independent.manager.SetLogger(logger); err != nil {
			return fmt.Errorf("manager.SetLogger: %w", err)
		}
	}

	return nil
}

// Name returns the unique name of the service
func (independent *Independent) Name() string {
	return independent.name
}

// Type returns the configuration type for an independent service.
func (independent *Independent) Type() config.Type {
	return config.IndependentType
}

// addDefaultServiceToTopology adds the default service config
// if no config was given for this service.
func (independent *Independent) addDefaultServiceToTopology() error {
	serviceConfig, err := independent.topologyHandler.Service(independent.name)
	if err == nil {
		return nil
	}

	serviceConfig = config.Service{
		Type:     config.IndependentType,
		Name:     independent.name,
		Handlers: []config.Handler{},
	}
	if err := fillDefaultModuleURL(&serviceConfig); err != nil {
		return err
	}
	if err := independent.topologyHandler.AddService(serviceConfig); err != nil {
		return fmt.Errorf("topologyHandler.AddService('%s'): %w", independent.name, err)
	}

	return nil
}

// addDefaultHandlerToTopology adds the default handler when no handlers exist.
// Unless there are handlers set by you or others
func (independent *Independent) addDefaultHandlerToTopology() error {
	serviceConfig, err := independent.topologyHandler.Service(independent.name)
	if err != nil {
		return fmt.Errorf("topologyHandler.Service('%s'): %w", independent.name, err)
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
	if err := independent.topologyHandler.SetService(serviceConfig); err != nil {
		return fmt.Errorf("topologyHandler.SetService('%s'): %w", independent.name, err)
	}

	return nil
}

// addServiceManagerToTopology stores a non-default manager handler when topology
// does not already have the same manager endpoint.
func (independent *Independent) addServiceManagerToTopology() error {
	// Our service's config in the topology.
	serviceConfig, err := independent.topologyHandler.Service(independent.name)
	if err != nil {
		return fmt.Errorf("topologyHandler.Service('%s'): %w", independent.name, err)
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
	if err := independent.topologyHandler.SetService(serviceConfig); err != nil {
		return fmt.Errorf("topologyHandler.SetService('%s'): %w", independent.name, err)
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
	serviceConfig, err := independent.topologyHandler.Service(independent.name)
	if err != nil {
		return fmt.Errorf("topologyHandler.Service('%s'): %w", independent.name, err)
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

	if err = independent.syncHandlerDepOutbounds(); err != nil {
		err = fmt.Errorf("syncHandlerDepOutbounds: %w", err)
		goto errOccurred
	}
	if err = independent.syncCommandOutbounds(); err != nil {
		err = fmt.Errorf("syncCommandOutbounds: %w", err)
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
		fmt.Printf("todo: implement inproc_topology for checking %d inproc managers\n", inprocServices)
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
			closeErr := independent.manager.StopService(independent.name)
			if closeErr != nil {
				err = fmt.Errorf("%v: manager.StopService: %w", err, closeErr)
			}
		}
	}

	return err
}

func (independent *Independent) syncHandlerDepOutbounds() error {
	serviceConfig, err := independent.topology.Service(independent.name)
	if err != nil {
		return fmt.Errorf("topologyClient.Service('%s'): %w", independent.name, err)
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
			proxyPointer := &dep.Proxies[proxyIndex]
			outbound, commandOutbounds, err := independent.handlerDepProxyOutboundTargets(serviceConfig, handler, dep.Proxies, proxyIndex, routes)
			if err != nil {
				return fmt.Errorf("handler %q proxy %q outbound: %w", dep.Name, proxyPointer.Name(), err)
			}
			if err := independent.syncHandlerDepProxyOutbounds(&serviceConfig, routes, proxyPointer, outbound, commandOutbounds); err != nil {
				return fmt.Errorf("handler %q proxy %q: %w", dep.Name, proxyPointer.Name(), err)
			}
		}
	}

	return nil
}

// startIpcServices starts IPC services this service depends on.
func (independent *Independent) startIpcServices() error {
	serviceConfig, err := independent.topology.Service(independent.name)
	if err != nil {
		return fmt.Errorf("topologyClient.Service('%s'): %w", independent.name, err)
	}

	startedRefs := make(map[string]struct{})
	return independent.startIpcServicesFor(serviceConfig, startedRefs)
}

func (independent *Independent) startIpcServicesFor(serviceConfig config.Service, startedRefs map[string]struct{}) error {
	for _, dep := range serviceConfig.HandlerDeps {
		for _, proxy := range dep.Proxies {
			if err := independent.startIpcService(proxy, startedRefs); err != nil {
				return fmt.Errorf("handler dep %q proxy %q: %w", dep.Name, proxy.Name(), err)
			}
		}
		for _, extension := range dep.Extensions {
			if err := independent.startIpcService(extension, startedRefs); err != nil {
				return fmt.Errorf("handler dep %q extension %q: %w", dep.Name, extension.Name(), err)
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
					return fmt.Errorf("handler %q command %q proxy %q: %w", handler.Category, dep.Name, proxy.Name(), err)
				}
			}
			for _, extension := range dep.Extensions {
				if err := independent.startIpcService(extension, startedRefs); err != nil {
					return fmt.Errorf("handler %q command %q extension %q: %w", handler.Category, dep.Name, extension.Name(), err)
				}
			}
		}
	}

	return nil
}

func (independent *Independent) startIpcService(pointer config.ServicePointer, startedRefs map[string]struct{}) error {
	if pointer.Ref != "" {
		serviceName, _ := pointer.RefPath()
		if serviceName == "" {
			return fmt.Errorf("dep ref %q is invalid", pointer.Ref)
		}
		if _, done := startedRefs[serviceName]; done {
			return nil
		}
		startedRefs[serviceName] = struct{}{}

		depService, err := independent.topology.Service(serviceName)
		if err != nil {
			return err
		}
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

	if pointer.Service.IsZero() {
		return fmt.Errorf("dep service pointer is empty")
	}
	if err := independent.startIpcServicesFor(pointer.Service, startedRefs); err != nil {
		return fmt.Errorf("service %q ipc deps: %w", pointer.Service.Name, err)
	}
	if !pointer.Service.IsIpc() {
		return nil
	}
	if len(pointer.Service.StartCommand) == 0 {
		return fmt.Errorf("service '%s' has no start command given", pointer.Service.Name)
	}
	managerHandler, err := pointer.Service.HandlerByCategory(topology.ServiceManagerCategory)
	if err != nil {
		return fmt.Errorf("service %q manager handler: %w", pointer.Service.Name, err)
	}
	handler, ok := managerHandler.AsIndependentHandler()
	if !ok {
		return fmt.Errorf("service %q manager handler is not an independent handler", pointer.Service.Name)
	}
	running, err := independent.manager.IsServiceRunningByManager(pointer.Service.Name, handler)
	if err != nil {
		return fmt.Errorf("manager.IsServiceRunningByManager('%s'): %w", pointer.Service.Name, err)
	}
	if running {
		return nil
	}
	if _, err := independent.manager.StartServiceByConfig(pointer.Service); err != nil {
		return fmt.Errorf("manager.StartServiceByConfig('%s'): %w", pointer.Service.Name, err)
	}
	return nil
}

// Validate protocol orders for all services and handlers in the topology:
// tcp can forward to tcp, but not other protocols.
// ipc can forward to ipc and tcp, but not inproc protocol.
// inproc can forward to inproc only, but not ipc or tcp protocol.
func (independent *Independent) validateProtocolOrders() error {
	serviceConfig, err := independent.topologyService(independent.name)
	if err != nil {
		return fmt.Errorf("service %q: %w", independent.name, err)
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

			for _, outbound := range proxyHandler.Outbounds {
				outboundService, outboundHandler, err := inlineOutboundServiceAndHandler(outbound)
				if err != nil {
					return fmt.Errorf("proxy %q handler %q outbound %q: %w", serviceConfig.Name, proxyHandler.Category, outbound.Name, err)
				}
				if err := validateProtocolOrder(serviceConfig, variant, outboundService, outboundHandler); err != nil {
					return fmt.Errorf("proxy %q handler %q outbound %q: %w", serviceConfig.Name, proxyHandler.Category, outbound.Name, err)
				}
			}
		}
	}

	for _, dep := range serviceConfig.HandlerDeps {
		for _, proxyPointer := range dep.Proxies {
			proxyService, _, err := independent.serviceAndHandlerFromPointer(proxyPointer)
			if err != nil {
				return fmt.Errorf("handler dep %q proxy %q: %w", dep.Name, proxyPointer.Name(), err)
			}
			if err := independent.validateProtocolOrdersFor(proxyService); err != nil {
				return fmt.Errorf("handler dep %q proxy %q: %w", dep.Name, proxyPointer.Name(), err)
			}
		}

		for _, extension := range dep.Extensions {
			extensionService, _, err := independent.serviceAndHandlerFromPointer(extension)
			if err != nil {
				return fmt.Errorf("handler dep %q extension %q: %w", dep.Name, extension.Name(), err)
			}
			if err := independent.validateProtocolOrdersFor(extensionService); err != nil {
				return fmt.Errorf("handler dep %q extension %q: %w", dep.Name, extension.Name(), err)
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
			for _, proxyPointer := range dep.Proxies {
				proxyService, _, err := independent.serviceAndHandlerFromPointer(proxyPointer)
				if err != nil {
					return fmt.Errorf("handler %q command %q proxy %q: %w", handler.Category, dep.Name, proxyPointer.Name(), err)
				}
				if err := independent.validateProtocolOrdersFor(proxyService); err != nil {
					return fmt.Errorf("handler %q command %q proxy %q: %w", handler.Category, dep.Name, proxyPointer.Name(), err)
				}
			}
			for _, extension := range dep.Extensions {
				extensionService, _, err := independent.serviceAndHandlerFromPointer(extension)
				if err != nil {
					return fmt.Errorf("handler %q command %q extension %q: %w", handler.Category, dep.Name, extension.Name(), err)
				}
				if err := independent.validateProtocolOrdersFor(extensionService); err != nil {
					return fmt.Errorf("handler %q command %q extension %q: %w", handler.Category, dep.Name, extension.Name(), err)
				}
			}
		}
	}

	return nil
}

func inlineOutboundServiceAndHandler(outbound config.Service) (config.Service, config.Handler, error) {
	if outbound.IsZero() {
		return config.Service{}, nil, fmt.Errorf("outbound service is empty")
	}
	handler, err := firstOutboundHandler(outbound)
	if err != nil {
		return config.Service{}, nil, err
	}
	return outbound, handler, nil
}

func (independent *Independent) serviceAndHandlerFromPointer(pointer config.ServicePointer) (config.Service, config.Handler, error) {
	if pointer.Ref == "" {
		if pointer.Service.IsZero() {
			return config.Service{}, nil, fmt.Errorf("service pointer is empty")
		}
		handler, err := firstOutboundHandler(pointer.Service)
		if err != nil {
			return config.Service{}, nil, err
		}
		return pointer.Service, handler, nil
	}

	serviceName, handlerCategory := pointer.RefPath()
	if serviceName == "" {
		return config.Service{}, nil, fmt.Errorf("ref %q is invalid", pointer.Ref)
	}
	if handlerCategory == "" {
		handlerCategory = handlers.DefaultHandlerCategory
	}

	serviceConfig, err := independent.topologyService(serviceName)
	if err != nil {
		return config.Service{}, nil, err
	}
	handlerVariant, err := serviceConfig.HandlerByCategory(handlerCategory)
	if err != nil {
		return config.Service{}, nil, fmt.Errorf("service %q handler %q: %w", serviceName, handlerCategory, err)
	}
	return serviceConfig, handlerVariant, nil
}

func (independent *Independent) topologyService(serviceName string) (config.Service, error) {
	if independent.topology != nil {
		return independent.topology.Service(serviceName)
	}
	if independent.topologyHandler != nil {
		return independent.topologyHandler.Service(serviceName)
	}
	return config.Service{}, fmt.Errorf("topology is nil")
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
	services, err := independent.topology.Services()
	if err != nil {
		return 0, err
	}

	inprocServices := 0
	for _, serviceConfig := range services {
		if err := independent.validateInprocServiceManagersFor(serviceConfig, &inprocServices); err != nil {
			return 0, err
		}
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
		for _, pointer := range dep.Proxies {
			if pointer.Ref != "" || pointer.Service.IsZero() {
				continue
			}
			if err := independent.validateInprocServiceManagersFor(pointer.Service, inprocServices); err != nil {
				return fmt.Errorf("handler dep %q proxy %q: %w", dep.Name, pointer.Name(), err)
			}
		}
		for _, pointer := range dep.Extensions {
			if pointer.Ref != "" || pointer.Service.IsZero() {
				continue
			}
			if err := independent.validateInprocServiceManagersFor(pointer.Service, inprocServices); err != nil {
				return fmt.Errorf("handler dep %q extension %q: %w", dep.Name, pointer.Name(), err)
			}
		}
	}

	for _, variant := range serviceConfig.Handlers {
		handler, ok := variant.AsIndependentHandler()
		if !ok {
			continue
		}
		for _, dep := range handler.CommandDeps {
			for _, pointer := range dep.Proxies {
				if pointer.Ref != "" || pointer.Service.IsZero() {
					continue
				}
				if err := independent.validateInprocServiceManagersFor(pointer.Service, inprocServices); err != nil {
					return fmt.Errorf("handler %q command %q proxy %q: %w", handler.Category, dep.Name, pointer.Name(), err)
				}
			}
			for _, pointer := range dep.Extensions {
				if pointer.Ref != "" || pointer.Service.IsZero() {
					continue
				}
				if err := independent.validateInprocServiceManagersFor(pointer.Service, inprocServices); err != nil {
					return fmt.Errorf("handler %q command %q extension %q: %w", handler.Category, dep.Name, pointer.Name(), err)
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
	serviceConfig, err := independent.topology.Service(independent.name)
	if err != nil {
		return fmt.Errorf("topologyClient.Service('%s'): %w", independent.name, err)
	}

	for handlerIndex := range serviceConfig.Handlers {
		handler, _ := serviceConfig.Handlers[handlerIndex].AsIndependentHandler()
		if handler.Category == topology.ServiceManagerCategory || len(handler.CommandDeps) == 0 {
			continue
		}

		for depIndex := range handler.CommandDeps {
			dep := &handler.CommandDeps[depIndex]
			for proxyIndex := range dep.Proxies {
				proxyPointer := &dep.Proxies[proxyIndex]
				var outbound config.Service
				// Get proxy target: either the next proxy or this service handler.
				if proxyIndex+1 < len(dep.Proxies) {
					var err error
					outbound, err = independent.proxyPointerOutboundTarget(dep.Proxies[proxyIndex+1])
					if err != nil {
						return fmt.Errorf("handler %q command %q proxy %q outbound: %w", handler.Category, dep.Name, proxyPointer.Name(), err)
					}
				} else {
					outbound = minimalOutboundService(serviceConfig, handler)
				}
				// Sync command proxy config: referenced root proxy or inline proxy embedded in this service tree.
				var err error
				if proxyPointer.Ref != "" {
					err = independent.syncReferencedCommandProxyOutbound(dep.Name, *proxyPointer, outbound)
				} else if proxyPointer.Service.IsZero() {
					err = fmt.Errorf("proxy service pointer is empty")
				} else {
					err = independent.syncInlineCommandProxyOutbound(&serviceConfig, dep.Name, proxyPointer, outbound)
				}
				if err != nil {
					return fmt.Errorf("handler %q command %q proxy %q: %w", handler.Category, dep.Name, proxyPointer.Name(), err)
				}
			}
		}
	}

	return nil
}

func (independent *Independent) handlerDepProxyOutboundTargets(serviceConfig config.Service, handlerConfig config.Handler, proxies []config.ServicePointer, proxyIndex int, routes []string) (config.Service, map[string]config.Service, error) {
	if proxyIndex+1 < len(proxies) {
		outbound, err := independent.proxyPointerOutboundTarget(proxies[proxyIndex+1])
		return outbound, nil, err
	}

	commandOutbounds := make(map[string]config.Service)
	for _, route := range routes {
		commandDep, ok := commandDepByName(handlerConfig, route)
		if !ok || len(commandDep.Proxies) == 0 {
			continue
		}
		outbound, err := independent.proxyPointerOutboundTarget(commandDep.Proxies[0])
		if err != nil {
			return config.Service{}, nil, fmt.Errorf("command %q first proxy: %w", route, err)
		}
		commandOutbounds[route] = outbound
	}

	return minimalOutboundService(serviceConfig, handlerConfig), commandOutbounds, nil
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

func (independent *Independent) syncHandlerDepProxyOutbounds(serviceConfig *config.Service, routes []string, proxyPointer *config.ServicePointer, outbound config.Service, commandOutbounds map[string]config.Service) error {
	if proxyPointer.Ref != "" {
		return independent.syncReferencedHandlerDepProxyOutbounds(routes, *proxyPointer, outbound, commandOutbounds)
	}
	if proxyPointer.Service.IsZero() {
		return fmt.Errorf("proxy service pointer is empty")
	}
	return independent.syncInlineHandlerDepProxyOutbounds(serviceConfig, routes, proxyPointer, outbound, commandOutbounds)
}

func minimalOutboundService(serviceConfig config.Service, handlerConfig config.Handler) config.Service {
	return config.Service{
		Type: serviceConfig.Type,
		Name: serviceConfig.Name,
		Handlers: []config.Handler{
			minimalOutboundHandler(handlerConfig),
		},
	}
}

func minimalOutboundHandler(handlerConfig config.Handler) config.IndependentHandler {
	handler, ok := handlerConfig.AsIndependentHandler()
	if !ok {
		return config.IndependentHandler{}
	}
	return config.IndependentHandler{
		Type:     handler.Type,
		Category: handler.Category,
		Endpoint: handler.Endpoint,
	}
}

func (independent *Independent) proxyPointerOutboundTarget(proxyPointer config.ServicePointer) (config.Service, error) {
	serviceConfig, handler, err := independent.serviceAndHandlerFromPointer(proxyPointer)
	if err != nil {
		return config.Service{}, err
	}
	return minimalOutboundService(serviceConfig, handler), nil
}

func firstOutboundHandler(serviceConfig config.Service) (config.Handler, error) {
	if len(serviceConfig.Handlers) == 0 {
		return nil, fmt.Errorf("proxy service %q has no handlers", serviceConfig.Name)
	}
	return serviceConfig.Handlers[0], nil
}

func (independent *Independent) syncInlineHandlerDepProxyOutbounds(serviceConfig *config.Service, routes []string, proxyPointer *config.ServicePointer, outbound config.Service, commandOutbounds map[string]config.Service) error {
	managerService, proxyConfig, updated, err := independent.updateInlineProxyHandlerConfig(serviceConfig, proxyPointer, func(proxyConfig config.ProxyHandler) (config.ProxyHandler, bool, error) {
		return configureHandlerDepProxyConfig(proxyConfig, routes, outbound, commandOutbounds)
	})
	if err != nil {
		return err
	}
	return applyProxyHandlerToManager(managerService, proxyConfig, updated)
}

func (independent *Independent) syncInlineCommandProxyOutbound(serviceConfig *config.Service, command string, proxyPointer *config.ServicePointer, outbound config.Service) error {
	managerService, proxyConfig, updated, err := independent.updateInlineProxyHandlerConfig(serviceConfig, proxyPointer, func(proxyConfig config.ProxyHandler) (config.ProxyHandler, bool, error) {
		updated := false
		if !containsString(proxyConfig.Routes, command) {
			proxyConfig.Routes = append(proxyConfig.Routes, command)
			updated = true
		}
		var updatedOutbound bool
		proxyConfig, updatedOutbound = ensureProxyHandlerOutbound(proxyConfig, outbound)
		updated = updated || updatedOutbound
		return proxyConfig, updated, nil
	})
	if err != nil {
		return err
	}
	return applyProxyHandlerToManager(managerService, proxyConfig, updated)
}

func (independent *Independent) syncReferencedHandlerDepProxyOutbounds(routes []string, proxyPointer config.ServicePointer, outbound config.Service, commandOutbounds map[string]config.Service) error {
	proxyServiceName, proxyHandlerCategory := proxyPointer.RefPath()
	if proxyServiceName == "" {
		return fmt.Errorf("proxy ref %q is invalid", proxyPointer.Ref)
	}
	if proxyHandlerCategory == "" {
		proxyHandlerCategory = handlers.DefaultHandlerCategory
	}

	proxyService, err := independent.topology.Service(proxyServiceName)
	if err != nil {
		return fmt.Errorf("topologyClient.Service('%s'): %w", proxyServiceName, err)
	}
	proxyHandlerVariant, err := proxyService.HandlerByCategory(proxyHandlerCategory)
	if err != nil {
		return fmt.Errorf("proxy service %q handler %q: %w", proxyServiceName, proxyHandlerCategory, err)
	}

	proxyConfig, ok := proxyHandlerVariant.AsProxyHandler()
	if !ok {
		return fmt.Errorf("proxy service %q handler %q is not a proxy handler", proxyServiceName, proxyHandlerCategory)
	}
	proxyConfig, updated, err := configureHandlerDepProxyConfig(proxyConfig, routes, outbound, commandOutbounds)
	if err != nil {
		return err
	}
	if updated {
		if err := independent.persistProxyHandlerConfig(proxyService, proxyConfig); err != nil {
			return err
		}
	}
	return applyProxyHandlerToManager(proxyService, proxyConfig, updated)
}

func (independent *Independent) syncReferencedCommandProxyOutbound(command string, proxyPointer config.ServicePointer, outbound config.Service) error {
	proxyServiceName, proxyHandlerCategory := proxyPointer.RefPath()
	if proxyServiceName == "" {
		return fmt.Errorf("proxy ref %q is invalid", proxyPointer.Ref)
	}
	if proxyHandlerCategory == "" {
		proxyHandlerCategory = handlers.DefaultHandlerCategory
	}

	proxyService, err := independent.topology.Service(proxyServiceName)
	if err != nil {
		return fmt.Errorf("topologyClient.Service('%s'): %w", proxyServiceName, err)
	}
	proxyHandlerVariant, err := proxyService.HandlerByCategory(proxyHandlerCategory)
	if err != nil {
		return fmt.Errorf("proxy service %q handler %q: %w", proxyServiceName, proxyHandlerCategory, err)
	}

	proxyConfig, ok := proxyHandlerVariant.AsProxyHandler()
	if !ok {
		return fmt.Errorf("proxy service %q handler %q is not a proxy handler", proxyServiceName, proxyHandlerCategory)
	}
	updated := false
	if !containsString(proxyConfig.Routes, command) {
		proxyConfig.Routes = append(proxyConfig.Routes, command)
		updated = true
	}
	proxyConfig, updatedOutbound := ensureProxyHandlerOutbound(proxyConfig, outbound)
	updated = updated || updatedOutbound
	var updatedForward bool
	proxyConfig, updatedForward, err = ensureProxyHandlerForward(proxyConfig, command, outbound)
	if err != nil {
		return err
	}
	updated = updated || updatedForward

	if updated {
		if err := independent.persistProxyHandlerConfig(proxyService, proxyConfig); err != nil {
			return err
		}
	}
	return applyProxyHandlerToManager(proxyService, proxyConfig, updated)
}

func firstProxyHandlerConfig(proxyService config.Service) (config.ProxyHandler, error) {
	for _, variant := range proxyService.Handlers {
		proxyHandler, ok := variant.AsProxyHandler()
		if ok {
			return proxyHandler, nil
		}
	}
	return config.ProxyHandler{}, fmt.Errorf("proxy service %q has no proxy handlers", proxyService.Name)
}

func (independent *Independent) updateInlineProxyHandlerConfig(serviceConfig *config.Service, proxyPointer *config.ServicePointer, configure func(config.ProxyHandler) (config.ProxyHandler, bool, error)) (config.Service, config.ProxyHandler, bool, error) {
	proxyConfig, err := firstProxyHandlerConfig(proxyPointer.Service)
	if err != nil {
		return config.Service{}, config.ProxyHandler{}, false, err
	}
	proxyConfig, updated, err := configure(proxyConfig)
	if err != nil {
		return config.Service{}, config.ProxyHandler{}, false, err
	}

	proxyPointer.Service.SetHandler(proxyConfig, true)
	if updated {
		if err := independent.topology.SetService(*serviceConfig); err != nil {
			return config.Service{}, config.ProxyHandler{}, false, fmt.Errorf("topologyClient.SetService('%s'): %w", serviceConfig.Name, err)
		}
	}
	return proxyPointer.Service, proxyConfig, updated, nil
}

func (independent *Independent) persistProxyHandlerConfig(proxyService config.Service, proxyConfig config.ProxyHandler) error {
	proxyService.SetHandler(proxyConfig, true)
	if err := independent.topology.SetService(proxyService); err != nil {
		return fmt.Errorf("topologyClient.SetService('%s'): %w", proxyService.Name, err)
	}
	return nil
}

func ensureProxyHandlerOutbound(proxyConfig config.ProxyHandler, outbound config.Service) (config.ProxyHandler, bool) {
	outbound = minimalOutbound(outbound)
	for i := range proxyConfig.Outbounds {
		if proxyConfig.Outbounds[i].Name != outbound.Name {
			continue
		}
		if servicesEqual(proxyConfig.Outbounds[i], outbound) {
			return proxyConfig, false
		}
		proxyConfig.Outbounds[i] = outbound
		return proxyConfig, true
	}

	proxyConfig.Outbounds = append(proxyConfig.Outbounds, outbound)
	return proxyConfig, true
}

func minimalOutbound(outbound config.Service) config.Service {
	if outbound.IsZero() || len(outbound.Handlers) == 0 {
		return outbound
	}
	return minimalOutboundService(outbound, outbound.Handlers[0])
}

func servicesEqual(a config.Service, b config.Service) bool {
	if a.Type != b.Type || a.Name != b.Name || a.ModuleUrl != b.ModuleUrl || a.StartCommand != b.StartCommand {
		return false
	}
	if len(a.HandlerDeps) != 0 || len(b.HandlerDeps) != 0 {
		return false
	}
	if len(a.Handlers) != len(b.Handlers) {
		return false
	}
	for i := range a.Handlers {
		if !handlersEqual(a.Handlers[i], b.Handlers[i]) {
			return false
		}
	}
	return true
}

func handlersEqual(a config.Handler, b config.Handler) bool {
	baseA, ok := a.AsIndependentHandler()
	if !ok {
		return false
	}
	baseB, ok := b.AsIndependentHandler()
	if !ok {
		return false
	}
	return baseA.Type == baseB.Type &&
		baseA.Category == baseB.Category &&
		baseA.Endpoint == baseB.Endpoint &&
		len(baseA.CommandDeps) == 0 &&
		len(baseB.CommandDeps) == 0
}

func configureHandlerDepProxyConfig(proxyConfig config.ProxyHandler, routes []string, outbound config.Service, commandOutbounds map[string]config.Service) (config.ProxyHandler, bool, error) {
	updated := false
	if !stringSlicesEqual(proxyConfig.Routes, routes) {
		proxyConfig.Routes = append([]string(nil), routes...)
		updated = true
	}

	var updatedOutbound bool
	proxyConfig, updatedOutbound = ensureProxyHandlerOutbound(proxyConfig, outbound)
	updated = updated || updatedOutbound

	for _, commandOutbound := range commandOutbounds {
		proxyConfig, updatedOutbound = ensureProxyHandlerOutbound(proxyConfig, commandOutbound)
		updated = updated || updatedOutbound
	}

	forwards := make(map[string]string, len(commandOutbounds))
	for command, commandOutbound := range commandOutbounds {
		outboundRef, err := proxyForwardRef(commandOutbound)
		if err != nil {
			return config.ProxyHandler{}, false, err
		}
		forwards[command] = outboundRef
	}
	if len(forwards) == 0 {
		forwards = nil
	}
	if !stringMapsEqual(proxyConfig.Forward, forwards) {
		proxyConfig.Forward = forwards
		updated = true
	}

	return proxyConfig, updated, nil
}

func ensureProxyHandlerForward(proxyConfig config.ProxyHandler, command string, outbound config.Service) (config.ProxyHandler, bool, error) {
	outboundRef, err := proxyForwardRef(outbound)
	if err != nil {
		return config.ProxyHandler{}, false, err
	}
	if proxyConfig.Forward == nil {
		proxyConfig.Forward = make(map[string]string)
	}
	if proxyConfig.Forward[command] == outboundRef {
		return proxyConfig, false, nil
	}
	proxyConfig.Forward[command] = outboundRef
	return proxyConfig, true, nil
}

func proxyForwardRef(outbound config.Service) (string, error) {
	if outbound.IsZero() {
		return "", fmt.Errorf("outbound service is empty")
	}
	if len(outbound.Handlers) == 0 {
		return "", fmt.Errorf("outbound service %q has no handlers", outbound.Name)
	}
	handler, ok := outbound.Handlers[0].AsIndependentHandler()
	if !ok {
		return "", fmt.Errorf("outbound service %q first handler is not an independent handler", outbound.Name)
	}
	return config.RefTarget(outbound.Name, handler.Category).Ref, nil
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

func stringMapsEqual(a map[string]string, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		if b[key] != value {
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

func applyProxyHandlerToManager(proxyService config.Service, proxyConfig config.ProxyHandler, updated bool) error {
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
	return independent.manager.StopService(independent.name)
}

func (independent *Independent) Wait() {
	if independent.blocker == nil {
		return
	}
	independent.blocker.Wait()
}
