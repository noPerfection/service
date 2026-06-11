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

// Return instance of an independent service.
// Optional parameters are name and topology config path.
func New(params ...interface{}) (*Independent, error) {
	name := DefaultName
	configPath := DefaultConfigPath
	managerEndpoint := DefaultServiceManagerEndpoint

	if len(params) > 3 {
		return nil, fmt.Errorf("too many arguments, expected name, config path, and manager endpoint")
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

	// Start the topology handler.
	topologyHandler, err := topology.NewHandler(configPath)
	if err != nil {
		return nil, fmt.Errorf("topology.NewHandler: %w", err)
	}

	if len(params) > 2 && params[2] != nil {
		managerEndpointArg, ok := params[2].(message.Endpoint)
		if !ok {
			return nil, fmt.Errorf("manager endpoint argument must be message.Endpoint")
		}
		managerEndpoint = managerEndpointArg
	} else {
		serviceConfig, err := topologyHandler.Service(name)
		if err == nil {
			managerHandler, err := serviceConfig.HandlerByCategory(topology.ServiceManagerCategory)
			if err == nil {
				managerEndpoint = managerHandler.AsHandler().Endpoint
			}
		}
	}

	m, err := manager.New(name, managerEndpoint)
	if err != nil {
		return nil, fmt.Errorf("manager.New: %w", err)
	}

	independent := &Independent{
		Handlers:              handlers.NewHandlers(),
		WithHardcodedTopology: NewHardcodedTopologies(name),
		topologyHandler:       topologyHandler,
		name:                  name,
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
		Type:      config.IndependentType,
		Name:      independent.name,
		ModuleUrl: DefaultModuleUrl,
		Handlers:  []config.HandlerVariant{},
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

	defaultHandler := config.Handler{
		Category: handlers.DefaultHandlerCategory,
		Endpoint: handlers.DefaultHandlerEndpoint,
		Type:     config.ReplierType,
	}
	serviceConfig.Handlers = []config.HandlerVariant{config.NewHandlerVariant(defaultHandler)}
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
		if currentManager.AsHandler().Endpoint == managerConfig.Endpoint {
			return nil
		}
	}
	if managerConfig.Endpoint == DefaultServiceManagerEndpoint {
		return nil
	}

	// Converting from the handler config format to the topology's config format.
	managerTopologyConfig := config.Handler{
		Type:     config.HandlerType(managerConfig.Type),
		Category: managerConfig.Category,
		Endpoint: managerConfig.Endpoint,
	}

	serviceConfig.SetHandler(config.NewHandlerVariant(managerTopologyConfig), true)
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
		configured := configuredVariant.AsHandler()
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
		handler := handlerVariant.AsHandler()
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
		handler := variant.AsHandler()
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
	running, err := independent.manager.IsServiceRunningByManager(pointer.Service.Name, managerHandler.AsHandler())
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

func (independent *Independent) syncCommandOutbounds() error {
	serviceConfig, err := independent.topology.Service(independent.name)
	if err != nil {
		return fmt.Errorf("topologyClient.Service('%s'): %w", independent.name, err)
	}

	for handlerIndex := range serviceConfig.Handlers {
		handlerVariant := &serviceConfig.Handlers[handlerIndex]
		handler := handlerVariant.AsHandler()
		if handler.Category == topology.ServiceManagerCategory || len(handler.CommandDeps) == 0 {
			continue
		}

		for depIndex := range handler.CommandDeps {
			dep := &handler.CommandDeps[depIndex]
			for proxyIndex := range dep.Proxies {
				proxyPointer := &dep.Proxies[proxyIndex]
				var outbound config.ServicePointer
				// Get proxy target: either the next proxy if it exists or this service handler if this is the last proxy.
				if proxyIndex+1 < len(dep.Proxies) {
					var err error
					outbound, err = independent.proxyPointerOutboundTarget(dep.Proxies[proxyIndex+1])
					if err != nil {
						return fmt.Errorf("handler %q command %q proxy %q outbound: %w", handler.Category, dep.Name, proxyPointer.Name(), err)
					}
				} else {
					outbound = commandOutboundTarget(serviceConfig, handler)
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

func (independent *Independent) handlerDepProxyOutboundTargets(serviceConfig config.Service, handlerConfig config.Handler, proxies []config.ServicePointer, proxyIndex int, routes []string) (config.ServicePointer, map[string]config.ServicePointer, error) {
	if proxyIndex+1 < len(proxies) {
		outbound, err := independent.proxyPointerOutboundTarget(proxies[proxyIndex+1])
		return outbound, nil, err
	}

	commandOutbounds := make(map[string]config.ServicePointer)
	for _, route := range routes {
		commandDep, ok := commandDepByName(handlerConfig, route)
		if !ok || len(commandDep.Proxies) == 0 {
			continue
		}
		outbound, err := independent.proxyPointerOutboundTarget(commandDep.Proxies[0])
		if err != nil {
			return config.ServicePointer{}, nil, fmt.Errorf("command %q first proxy: %w", route, err)
		}
		commandOutbounds[route] = outbound
	}

	return commandOutboundTarget(serviceConfig, handlerConfig), commandOutbounds, nil
}

func commandDepByName(handlerConfig config.Handler, command string) (config.DepService, bool) {
	for _, dep := range handlerConfig.CommandDeps {
		if dep.Name == command {
			return dep, true
		}
	}
	return config.DepService{}, false
}

func (independent *Independent) syncHandlerDepProxyOutbounds(serviceConfig *config.Service, routes []string, proxyPointer *config.ServicePointer, outbound config.ServicePointer, commandOutbounds map[string]config.ServicePointer) error {
	if proxyPointer.Ref != "" {
		return independent.syncReferencedHandlerDepProxyOutbounds(routes, *proxyPointer, outbound, commandOutbounds)
	}
	if proxyPointer.Service.IsZero() {
		return fmt.Errorf("proxy service pointer is empty")
	}
	return independent.syncInlineHandlerDepProxyOutbounds(serviceConfig, routes, proxyPointer, outbound, commandOutbounds)
}

func commandOutboundTarget(serviceConfig config.Service, handlerConfig config.Handler) config.ServicePointer {
	return config.ServiceTarget(minimalOutboundService(serviceConfig, handlerConfig))
}

func minimalOutboundService(serviceConfig config.Service, handlerConfig config.Handler) config.Service {
	return config.Service{
		Type:     serviceConfig.Type,
		Name:     serviceConfig.Name,
		Handlers: config.NewHandlerVariants(minimalOutboundHandler(handlerConfig)),
	}
}

func minimalOutboundHandler(handlerConfig config.Handler) config.Handler {
	return config.Handler{
		Type:     handlerConfig.Type,
		Category: handlerConfig.Category,
		Endpoint: handlerConfig.Endpoint,
	}
}

func (independent *Independent) proxyPointerOutboundTarget(proxyPointer config.ServicePointer) (config.ServicePointer, error) {
	if proxyPointer.Ref == "" {
		if proxyPointer.Service.IsZero() {
			return config.ServicePointer{}, fmt.Errorf("proxy service pointer is empty")
		}
		handler, err := firstOutboundHandler(proxyPointer.Service)
		if err != nil {
			return config.ServicePointer{}, err
		}
		return config.ServiceTarget(minimalOutboundService(proxyPointer.Service, handler)), nil
	}

	proxyServiceName, proxyHandlerCategory := proxyPointer.RefPath()
	if proxyServiceName == "" {
		return config.ServicePointer{}, fmt.Errorf("proxy ref %q is invalid", proxyPointer.Ref)
	}
	if proxyHandlerCategory == "" {
		proxyHandlerCategory = handlers.DefaultHandlerCategory
	}

	proxyService, err := independent.topology.Service(proxyServiceName)
	if err != nil {
		return config.ServicePointer{}, fmt.Errorf("topologyClient.Service('%s'): %w", proxyServiceName, err)
	}
	proxyHandlerVariant, err := proxyService.HandlerByCategory(proxyHandlerCategory)
	if err != nil {
		return config.ServicePointer{}, fmt.Errorf("proxy service %q handler %q: %w", proxyService.Name, proxyHandlerCategory, err)
	}

	return config.ServiceTarget(minimalOutboundService(proxyService, proxyHandlerVariant.AsHandler())), nil
}

func firstOutboundHandler(serviceConfig config.Service) (config.Handler, error) {
	if len(serviceConfig.Handlers) == 0 {
		return config.Handler{}, fmt.Errorf("proxy service %q has no handlers", serviceConfig.Name)
	}
	return serviceConfig.Handlers[0].AsHandler(), nil
}

func (independent *Independent) syncInlineHandlerDepProxyOutbounds(serviceConfig *config.Service, routes []string, proxyPointer *config.ServicePointer, outbound config.ServicePointer, commandOutbounds map[string]config.ServicePointer) error {
	managerService, proxyConfig, updated, err := independent.updateInlineProxyHandlerConfig(serviceConfig, proxyPointer, func(proxyConfig config.ProxyHandler) (config.ProxyHandler, bool, error) {
		return configureHandlerDepProxyConfig(proxyConfig, routes, outbound, commandOutbounds)
	})
	if err != nil {
		return err
	}
	return applyProxyHandlerToManager(managerService, proxyConfig, updated)
}

func (independent *Independent) syncInlineCommandProxyOutbound(serviceConfig *config.Service, command string, proxyPointer *config.ServicePointer, outbound config.ServicePointer) error {
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

func (independent *Independent) syncReferencedHandlerDepProxyOutbounds(routes []string, proxyPointer config.ServicePointer, outbound config.ServicePointer, commandOutbounds map[string]config.ServicePointer) error {
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

	proxyConfig := proxyHandlerVariant.AsProxyHandler()
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

func (independent *Independent) syncReferencedCommandProxyOutbound(command string, proxyPointer config.ServicePointer, outbound config.ServicePointer) error {
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

	proxyConfig := proxyHandlerVariant.AsProxyHandler()
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
		if variant.ProxyHandler != nil {
			return variant.AsProxyHandler(), nil
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

	proxyPointer.Service.SetHandler(config.NewProxyHandlerVariant(proxyConfig), true)
	if updated {
		if err := independent.topology.SetService(*serviceConfig); err != nil {
			return config.Service{}, config.ProxyHandler{}, false, fmt.Errorf("topologyClient.SetService('%s'): %w", serviceConfig.Name, err)
		}
	}
	return proxyPointer.Service, proxyConfig, updated, nil
}

func (independent *Independent) persistProxyHandlerConfig(proxyService config.Service, proxyConfig config.ProxyHandler) error {
	proxyService.SetHandler(config.NewProxyHandlerVariant(proxyConfig), true)
	if err := independent.topology.SetService(proxyService); err != nil {
		return fmt.Errorf("topologyClient.SetService('%s'): %w", proxyService.Name, err)
	}
	return nil
}

func ensureProxyHandlerOutbound(proxyConfig config.ProxyHandler, outbound config.ServicePointer) (config.ProxyHandler, bool) {
	outbound = minimalOutboundPointer(outbound)
	for i := range proxyConfig.Outbounds {
		if proxyConfig.Outbounds[i].Name() != outbound.Name() {
			continue
		}
		if outbound.Ref != "" {
			if proxyConfig.Outbounds[i].Ref == outbound.Ref {
				return proxyConfig, false
			}
			proxyConfig.Outbounds[i] = outbound
			return proxyConfig, true
		}
		if proxyConfig.Outbounds[i].Service.IsZero() {
			proxyConfig.Outbounds[i] = outbound
			return proxyConfig, true
		}
		if servicePointersEqual(proxyConfig.Outbounds[i], outbound) {
			return proxyConfig, false
		}
		proxyConfig.Outbounds[i] = outbound
		return proxyConfig, true
	}

	proxyConfig.Outbounds = append(proxyConfig.Outbounds, outbound)
	return proxyConfig, true
}

func minimalOutboundPointer(outbound config.ServicePointer) config.ServicePointer {
	if outbound.Ref != "" || outbound.Service.IsZero() || len(outbound.Service.Handlers) == 0 {
		return outbound
	}
	return config.ServiceTarget(minimalOutboundService(outbound.Service, outbound.Service.Handlers[0].AsHandler()))
}

func servicePointersEqual(a config.ServicePointer, b config.ServicePointer) bool {
	if a.Ref != b.Ref {
		return false
	}
	if a.Ref != "" {
		return true
	}
	return servicesEqual(a.Service, b.Service)
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
		if !handlersEqual(a.Handlers[i].AsHandler(), b.Handlers[i].AsHandler()) {
			return false
		}
	}
	return true
}

func handlersEqual(a config.Handler, b config.Handler) bool {
	return a.Type == b.Type &&
		a.Category == b.Category &&
		a.Endpoint == b.Endpoint &&
		len(a.CommandDeps) == 0 &&
		len(b.CommandDeps) == 0
}

func configureHandlerDepProxyConfig(proxyConfig config.ProxyHandler, routes []string, outbound config.ServicePointer, commandOutbounds map[string]config.ServicePointer) (config.ProxyHandler, bool, error) {
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

func ensureProxyHandlerForward(proxyConfig config.ProxyHandler, command string, outbound config.ServicePointer) (config.ProxyHandler, bool, error) {
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

func proxyForwardRef(outbound config.ServicePointer) (string, error) {
	if outbound.Ref != "" {
		serviceName, handlerCategory := outbound.RefPath()
		if serviceName == "" {
			return "", fmt.Errorf("outbound ref %q is invalid", outbound.Ref)
		}
		return config.RefTarget(serviceName, handlerCategory).Ref, nil
	}
	if outbound.Service.IsZero() {
		return "", fmt.Errorf("outbound service pointer is empty")
	}
	if len(outbound.Service.Handlers) == 0 {
		return "", fmt.Errorf("outbound service %q has no handlers", outbound.Service.Name)
	}
	handlerCategory := outbound.Service.Handlers[0].AsHandler().Category
	return config.RefTarget(outbound.Service.Name, handlerCategory).Ref, nil
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
		endpoint = managerHandler.AsHandler().Endpoint
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
	if !updated {
		return nil
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
