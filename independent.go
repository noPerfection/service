// Package service is the primary service.
// This package is calling out the orchestra. Then within that orchestra sets up
// - handler manager
// - proxies
// - extensions
// - config manager
// - dep manager
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
	if err := independent.topologyHandler.SetService(serviceConfig); err != nil {
		return fmt.Errorf("topologyHandler.SetService('%s'): %w", independent.name, err)
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
	fmt.Println("warning: Topology might have dependency per command/handler to other services. But its not launched nor piped consider doing it")

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

	if err = independent.syncCommandOutbounds(); err != nil {
		err = fmt.Errorf("syncCommandOutbounds: %w", err)
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

// Set the outbound of the proxy to this service
func (independent *Independent) syncCommandOutbounds() error {
	topologyClient, err := topology.NewClient()
	if err != nil {
		return fmt.Errorf("topology.NewClient: %w", err)
	}
	defer topologyClient.Close()

	serviceConfig, err := topologyClient.Service(independent.name)
	if err != nil {
		return fmt.Errorf("topologyClient.Service('%s'): %w", independent.name, err)
	}

	for _, variant := range serviceConfig.Handlers {
		handler := variant.AsHandler()
		if handler.Category == topology.ServiceManagerCategory || len(handler.CommandDeps) == 0 {
			continue
		}

		for _, dep := range handler.CommandDeps {
			for proxyIndex, proxyPointer := range dep.Proxies {
				outbound, err := independent.commandProxyOutboundTarget(topologyClient, serviceConfig, handler, dep.Proxies, proxyIndex)
				if err != nil {
					return fmt.Errorf("handler %q command %q proxy %q outbound: %w", handler.Category, dep.Name, proxyPointer.Name(), err)
				}
				if err := independent.syncCommandProxyOutbound(topologyClient, dep.Name, proxyPointer, outbound); err != nil {
					return fmt.Errorf("handler %q command %q proxy %q: %w", handler.Category, dep.Name, proxyPointer.Name(), err)
				}
			}
		}
	}

	return nil
}

func (independent *Independent) syncCommandProxyOutbound(topologyClient *topology.Client, command string, proxyPointer config.ServicePointer, outbound config.ServicePointer) error {
	if proxyPointer.Ref != "" {
		return independent.syncReferencedCommandProxyOutbound(topologyClient, command, proxyPointer, outbound)
	}
	if proxyPointer.Service.IsZero() {
		return fmt.Errorf("proxy service pointer is empty")
	}
	return independent.syncInlineCommandProxyOutbound(topologyClient, command, proxyPointer.Service, outbound)
}

func (independent *Independent) commandProxyOutboundTarget(topologyClient *topology.Client, serviceConfig config.Service, handlerConfig config.Handler, proxies []config.ServicePointer, proxyIndex int) (config.ServicePointer, error) {
	if proxyIndex+1 >= len(proxies) {
		return commandOutboundTarget(serviceConfig, handlerConfig), nil
	}
	return independent.proxyPointerOutboundTarget(topologyClient, proxies[proxyIndex+1])
}

func commandOutboundTarget(serviceConfig config.Service, handlerConfig config.Handler) config.ServicePointer {
	outboundService := serviceConfig
	outboundService.Handlers = config.NewHandlerVariants(handlerConfig)
	return config.ServiceTarget(outboundService)
}

func (independent *Independent) proxyPointerOutboundTarget(topologyClient *topology.Client, proxyPointer config.ServicePointer) (config.ServicePointer, error) {
	if proxyPointer.Ref == "" {
		if proxyPointer.Service.IsZero() {
			return config.ServicePointer{}, fmt.Errorf("proxy service pointer is empty")
		}
		return proxyPointer, nil
	}

	proxyServiceName, proxyHandlerCategory := proxyPointer.RefPath()
	if proxyServiceName == "" {
		return config.ServicePointer{}, fmt.Errorf("proxy ref %q is invalid", proxyPointer.Ref)
	}
	if proxyHandlerCategory == "" {
		proxyHandlerCategory = handlers.DefaultHandlerCategory
	}

	proxyService, err := topologyClient.Service(proxyServiceName)
	if err != nil {
		return config.ServicePointer{}, fmt.Errorf("topologyClient.Service('%s'): %w", proxyServiceName, err)
	}
	proxyHandlerVariant, err := proxyService.HandlerByCategory(proxyHandlerCategory)
	if err != nil {
		return config.ServicePointer{}, fmt.Errorf("proxy service %q handler %q: %w", proxyServiceName, proxyHandlerCategory, err)
	}
	proxyService.Handlers = []config.HandlerVariant{proxyHandlerVariant}

	return config.ServiceTarget(proxyService), nil
}

func (independent *Independent) syncInlineCommandProxyOutbound(topologyClient *topology.Client, command string, proxyService config.Service, outbound config.ServicePointer) error {
	proxyConfig, err := firstProxyHandlerConfig(proxyService)
	if err != nil {
		return err
	}
	proxyConfig.Routes = appendUnique(proxyConfig.Routes, command)
	proxyConfig, _ = ensureProxyHandlerOutbound(proxyConfig, outbound)

	managerService := proxyService
	if topologyService, err := topologyClient.Service(proxyService.Name); err == nil {
		managerService = topologyService
	}
	if err := persistProxyHandlerConfig(topologyClient, managerService, proxyConfig); err != nil {
		return err
	}
	proxyManagerClient, err := newProxyManagerClient(managerService)
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

func (independent *Independent) syncReferencedCommandProxyOutbound(topologyClient *topology.Client, command string, proxyPointer config.ServicePointer, outbound config.ServicePointer) error {
	proxyServiceName, proxyHandlerCategory := proxyPointer.RefPath()
	if proxyServiceName == "" {
		return fmt.Errorf("proxy ref %q is invalid", proxyPointer.Ref)
	}
	if proxyHandlerCategory == "" {
		proxyHandlerCategory = handlers.DefaultHandlerCategory
	}

	proxyService, err := topologyClient.Service(proxyServiceName)
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
		if err := persistProxyHandlerConfig(topologyClient, proxyService, proxyConfig); err != nil {
			return err
		}
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

func firstProxyHandlerConfig(proxyService config.Service) (config.ProxyHandler, error) {
	for _, variant := range proxyService.Handlers {
		if variant.ProxyHandler != nil {
			return variant.AsProxyHandler(), nil
		}
	}
	return config.ProxyHandler{}, fmt.Errorf("proxy service %q has no proxy handlers", proxyService.Name)
}

func persistProxyHandlerConfig(topologyClient *topology.Client, proxyService config.Service, proxyConfig config.ProxyHandler) error {
	proxyService.SetHandler(config.NewProxyHandlerVariant(proxyConfig), true)
	if err := topologyClient.SetService(proxyService); err != nil {
		return fmt.Errorf("topologyClient.SetService('%s'): %w", proxyService.Name, err)
	}
	return nil
}

func ensureProxyHandlerOutbound(proxyConfig config.ProxyHandler, outbound config.ServicePointer) (config.ProxyHandler, bool) {
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
		changed := ensureServiceHasHandlers(&proxyConfig.Outbounds[i].Service, outbound.Service.Handlers)
		return proxyConfig, changed
	}

	proxyConfig.Outbounds = append(proxyConfig.Outbounds, outbound)
	return proxyConfig, true
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

func ensureServiceHasHandlers(serviceConfig *config.Service, handlersToAdd []config.HandlerVariant) bool {
	changed := false
	for _, handlerToAdd := range handlersToAdd {
		category := handlerToAdd.AsHandler().Category
		if _, err := serviceConfig.HandlerByCategory(category); err == nil {
			continue
		}
		serviceConfig.Handlers = append(serviceConfig.Handlers, handlerToAdd)
		changed = true
	}
	return changed
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
