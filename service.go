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
	"slices"
	"sync"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/log"
	clientConfig "github.com/noPerfection/protocol/client/config"
	"github.com/noPerfection/protocol/handler/base"
	"github.com/noPerfection/protocol/handler/manager_client"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/runtime"
	serviceConfig "github.com/noPerfection/runtime/config/service"
	"github.com/noPerfection/service/manager"
)

const DefaultName = "main"
const DefaultRuntimeEndpoint = message.NewEndpoint("main_runtime", 0)
const DefaultConfigPath = "noPerfection.json"

// Independent keeps all necessary parameters of the independent service.
type Independent struct {
	runtimeHandler     *runtime.Handler // runtime handles the configuration and dependencies
	Handlers           datatype.KeyValue
	RequiredExtensions datatype.KeyValue
	Logger             *log.Logger
	name               string
	// The blocker is a shutdown latch:
	// it keeps the process alive after Start() returns, and it unblocks when the service is fully closed.
	blocker *sync.WaitGroup
	manager *manager.Manager // manage this service from other parts
}

// New service.
// Optional parameters are name, config path, and runtime endpoint.
//
// It will also create the context internally and start it.
func New(params ...interface{}) (*Independent, error) {
	name := DefaultName
	configPath := DefaultConfigPath
	runtimeEndpoint := DefaultRuntimeEndpoint

	if len(params) > 3 {
		return nil, fmt.Errorf("too many arguments, expected name, config path, and runtime endpoint")
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
	if len(params) > 2 && params[2] != nil {
		endpointArg, ok := params[2].(message.Endpoint)
		if !ok {
			return nil, fmt.Errorf("runtime endpoint argument must be message.Endpoint")
		}
		runtimeEndpoint = endpointArg
	}

	// Start the runtime
	runtimeHandler, err := runtime.NewHandler(configPath, runtimeEndpoint)
	if err != nil {
		return nil, fmt.Errorf("runtime.NewHandler: %w", err)
	}

	independent := &Independent{
		runtimeHandler: runtimeHandler,
		Handlers:       datatype.New(),
		name:           name,
		blocker:        nil,
	}

	logger, err := log.New(name, true)
	if err != nil {
		err = fmt.Errorf("log.New(%s): %w", name, err)

		return nil, err
	}
	independent.Logger = logger

	return independent, nil
}

// SetHandler of category
//
// Todo change to keep the handlers by their id.
func (independent *Independent) SetHandler(category string, controller base.Interface) {
	independent.Handlers.Set(category, controller)
}

// Context returns the runtime context owned by the service.
func (independent *Independent) Context() *runtime.Handler {
	return independent.runtimeHandler
}

// Name returns the unique name of the service
func (independent *Independent) Name() string {
	return independent.name
}

// Type returns the configuration type for an independent service.
func (independent *Independent) Type() serviceConfig.Type {
	return serviceConfig.IndependentType
}

// SetConfig prepares and stores the generated service configuration.
func (independent *Independent) SetConfig() error {
	return independent.setConfig()
}

// SetProxyUnitsBy stores proxy units for the given destination rule.
func (independent *Independent) SetProxyUnitsBy(dest *serviceConfig.Rule) error {
	return independent.setProxyUnitsBy(dest)
}

// SetProxyChain adds a proxy chain to the list of proxy chains to set.
//
// The proxies are managed by the proxy handler in the context.
// This method creates a serviceConfig.ProxyChain.
// Then send it to the proxy handler.
func (independent *Independent) SetProxyChain(params ...interface{}) error {
	if len(params) < 1 || len(params) > 3 {
		return fmt.Errorf("argument amount is invalid, either one or three arguments must be set")
	}
	if independent.runtimeHandler == nil || !independent.runtimeHandler.IsConfigRunning() {
		return fmt.Errorf("context or config engine is not running")
	}

	independent.runtimeHandler.SetService(independent.name, independent.name)

	if !independent.runtimeHandler.IsDepManagerRunning() {
		err := independent.runtimeHandler.StartDepManager()
		if err != nil {
			return fmt.Errorf("runtimeHandler.StartDepManager: %w", err)
		}

	}

	if !independent.runtimeHandler.IsProxyHandlerRunning() {
		err := independent.runtimeHandler.StartProxyHandler()
		if err != nil {
			return fmt.Errorf("runtimeHandler.StartProxyHandler: %w", err)
		}
	}

	var proxyChain *serviceConfig.ProxyChain
	var ok bool

	if len(params) == 1 {
		proxyChain, ok = params[0].(*serviceConfig.ProxyChain)
		if !ok {
			return fmt.Errorf("given a one parameter it must be of *parent.ProxyChain type")
		}
		if len(proxyChain.Destination.Urls) == 0 {
			proxyChain.Destination.Urls = []string{independent.name}
		}
		if !proxyChain.IsValid() {
			return fmt.Errorf("given a one parameter, the proxy chain is not valid")
		}
	} else {
		var err error
		proxyChain, err = serviceConfig.NewProxyChain(params...)
		if err != nil {
			return fmt.Errorf("serviceConfig.NewProxyChain: %w", err)
		}
		if len(proxyChain.Destination.Urls) == 0 {
			proxyChain.Destination.Urls = []string{independent.name}
		}
		if !proxyChain.IsValid() {
			return fmt.Errorf("given proxy chain fields, the proxy chain is not valid")
		}
	}

	proxyClient := independent.runtimeHandler.ProxyClient()
	if err := proxyClient.Set(proxyChain); err != nil {
		return fmt.Errorf("independent.runtimeHandler.Set('proxyChain'): %w", err)
	}

	return nil
}

// RequireExtension lints the id to the extension url
func (independent *Independent) RequireExtension(id string, url string) {
	if independent.RequiredExtensions.Exist(id) {
		independent.RequiredExtensions.Set(id, url)
	}
}

func (independent *Independent) requiredControllerExtensions() []string {
	var extensions []string
	for _, controllerInterface := range independent.Handlers {
		c := controllerInterface.(base.Interface)
		extensions = append(extensions, c.DepIds()...)
	}

	return extensions
}

// The generateConfig sends a signal to the context to generate a new configuration for this service.
// The method requests multiple commands. One command to generate a service configuration.
// Then a request to generate a handler configurations.
//
// The generated configuration returned back.
func (independent *Independent) generateConfig() (*serviceConfig.Service, error) {
	configClient := independent.runtimeHandler.Config()

	serviceType := independent.Type()
	generatedConfig, err := configClient.GenerateService(independent.name, independent.name, serviceType)
	if err != nil {
		return nil, fmt.Errorf("configClient.GenerateService('%s', '%s', '%s'): %w", independent.name, independent.name, serviceType, err)
	}
	generatedConfig.Manager.UrlFunc(clientConfig.Url)

	// Get all handlers and add them into the service
	for category, raw := range independent.Handlers {
		handler := raw.(base.Interface)
		generatedHandler, err := configClient.GenerateHandler(handler.Type(), category, false)
		if err != nil {
			return nil, fmt.Errorf("configClient.GenerateHandler('%s', '%s', internal: false): %w", handler.Type(), category, err)
		}

		handler.SetConfig(generatedHandler)

		generatedConfig.SetHandler(generatedHandler)
	}

	// Some handlers were generated and added into generated service config.
	// Notify the config engine to update the service.
	if err := configClient.SetService(generatedConfig); err != nil {
		return nil, fmt.Errorf("configClient.SetService('generated'): %w", err)
	}

	return generatedConfig, nil
}

// lintConfig gets the configuration from the context and sets them in the service and handler.
func (independent *Independent) lintConfig() error {
	configClient := independent.runtimeHandler.Config()

	returnedService, err := configClient.Service(independent.name)
	if err != nil {
		return fmt.Errorf("configClient.Service('%s', '%s'): %w", independent.name, independent.Type(), err)
	}
	returnedService.Manager.UrlFunc(clientConfig.Url)

	if returnedService.Type != independent.Type() {
		return fmt.Errorf("configClient.Service('%s') returned type '%s', expected '%s'", independent.name, returnedService.Type, independent.Type())
	}

	for category, raw := range independent.Handlers {
		handler := raw.(base.Interface)

		returnedHandler, err := returnedService.HandlerByCategory(category)
		if err != nil {
			generatedHandler, err := configClient.GenerateHandler(handler.Type(), category, false)
			if err != nil {
				return fmt.Errorf("configClient.GenerateHandler('%s', '%s', internal: false): %w", handler.Type(), category, err)
			}

			handler.SetConfig(generatedHandler)

			returnedService.SetHandler(generatedHandler)
			if err := configClient.SetService(returnedService); err != nil {
				return fmt.Errorf("configClient.SetService('returned'): %w", err)
			}
		} else {
			handler.SetConfig(returnedHandler)
		}
	}

	return nil
}

// The setConfig sets the configuration of this service and handlers.
// If the configuration doesn't exist, generates the service and handler.
// The returned configuration from the context is linted into service and handler.
//
// Important node. This method doesn't set the proxies or extensions.
func (independent *Independent) setConfig() error {
	configClient := independent.runtimeHandler.Config()

	// prepare the configuration
	exist, err := configClient.ServiceExist(independent.name)
	if err != nil {
		return fmt.Errorf("configClient.ServiceExist('%s'): %w", independent.name, err)
	}

	if !exist {
		_, err := independent.generateConfig()
		if err != nil {
			return fmt.Errorf("generateConfig: %w", err)
		}

		return nil
	}

	if err = independent.lintConfig(); err != nil {
		return fmt.Errorf("lintConfig: %w", err)
	}

	return nil
}

func (independent *Independent) setProxyUnitsBy(dest *serviceConfig.Rule) error {
	proxyClient := independent.runtimeHandler.ProxyClient()

	if dest.IsRoute() {
		units := independent.unitsByRouteRule(dest)
		if err := proxyClient.SetUnits(dest, units); err != nil {
			return fmt.Errorf("proxyClient.SetUnits: %w", err)
		}
	} else if dest.IsHandler() {
		units := independent.unitsByHandlerRule(dest)
		if err := proxyClient.SetUnits(dest, units); err != nil {
			return fmt.Errorf("proxyClient.SetUnits: %w", err)
		}
	} else if dest.IsService() {
		units := independent.unitsByServiceRule(dest)
		if err := proxyClient.SetUnits(dest, units); err != nil {
			return fmt.Errorf("proxyClient.SetUnits: %w", err)
		}
	}

	return nil
}

// The setProxyUnits gets the list of proxy chains for this service.
// Then, it creates a proxy units.
// Todo if the extension is sending a ready command, then update the command list.
func (independent *Independent) setProxyUnits() error {
	proxyClient := independent.runtimeHandler.ProxyClient()
	proxyChains, err := proxyClient.ProxyChains()
	if err != nil {
		return fmt.Errorf("proxyClient.ProxyChainsByRuleUrl: %w", err)
	}

	// set the proxy destination units for each rule
	for _, proxyChain := range proxyChains {
		dest := proxyChain.Destination
		if err := independent.setProxyUnitsBy(dest); err != nil {
			return fmt.Errorf("independent.setProxyUnitsBy(rule='%v'): %w", dest, err)
		}
	}

	return nil
}

// unitsByRouteRule returns the list of units for the route rule
func (independent *Independent) unitsByRouteRule(rule *serviceConfig.Rule) []*serviceConfig.Unit {
	units := make([]*serviceConfig.Unit, 0, len(rule.Commands)*len(rule.Categories))

	if len(independent.Handlers) == 0 {
		return units
	}

	for _, raw := range independent.Handlers {
		handlerInterface := raw.(base.Interface)
		hConfig := handlerInterface.Config()

		if !slices.Contains(rule.Categories, hConfig.Category) {
			continue
		}

		for _, command := range rule.Commands {
			if slices.Contains(rule.ExcludedCommands, command) {
				continue
			}

			if !handlerInterface.IsRouteExist(command) {
				continue
			}

			unit := &serviceConfig.Unit{
				ServiceId: independent.name,
				HandlerId: hConfig.Id,
				Command:   command,
			}

			units = append(units, unit)
		}
	}

	return units
}

// unitsByHandlerRule returns the list of units for the handler rule
func (independent *Independent) unitsByHandlerRule(rule *serviceConfig.Rule) []*serviceConfig.Unit {
	units := make([]*serviceConfig.Unit, 0, len(rule.Categories))

	for _, raw := range independent.Handlers {
		handlerInterface := raw.(base.Interface)
		hConfig := handlerInterface.Config()

		if !slices.Contains(rule.Categories, hConfig.Category) {
			continue
		}

		commands := handlerInterface.RouteCommands()

		for _, command := range commands {
			if slices.Contains(rule.ExcludedCommands, command) {
				continue
			}

			unit := &serviceConfig.Unit{
				ServiceId: independent.name,
				HandlerId: hConfig.Id,
				Command:   command,
			}

			units = append(units, unit)
		}
	}

	return units
}

// unitsByServiceRule returns the list of units for the service rule
func (independent *Independent) unitsByServiceRule(rule *serviceConfig.Rule) []*serviceConfig.Unit {
	units := make([]*serviceConfig.Unit, 0, len(rule.Categories))

	for _, raw := range independent.Handlers {
		handlerInterface := raw.(base.Interface)
		hConfig := handlerInterface.Config()

		commands := handlerInterface.RouteCommands()

		for _, command := range commands {
			if slices.Contains(rule.ExcludedCommands, command) {
				continue
			}

			unit := &serviceConfig.Unit{
				ServiceId: independent.name,
				HandlerId: hConfig.Id,
				Command:   command,
			}

			units = append(units, unit)
		}
	}

	return units
}

// newManager creates a manager.Manager and assigns it to manager, otherwise manager is nil.
//
// The manager.Manager depends on config set by setConfig.
//
// The manager.Manager depends on Logger, set automatically.
//
// This function lints manager.Manager with runtime handler.
func (independent *Independent) newManager() error {
	m, err := manager.New(independent.runtimeHandler, independent.name, &independent.blocker)
	if err != nil {
		return fmt.Errorf("manager.New: %w", err)
	}
	err = m.SetLogger(independent.Logger)
	if err != nil {
		return fmt.Errorf("manager.SetLogger: %w", err)
	}
	independent.manager = m

	return nil
}

// setHandlerClient creates a handler manager clients and sets them into the service manager.
func (independent *Independent) setHandlerClient(c base.Interface) error {
	handlerClient, err := manager_client.New(c.Config())
	if err != nil {
		return fmt.Errorf("manager_client.New('%s'): %w", c.Config().Category, err)
	}
	independent.manager.SetHandlerManagers([]manager_client.Interface{handlerClient})

	return nil
}

// startHandler sets the log into the handler which is prepared already.
// Then, starts it.
func (independent *Independent) startHandler(handler base.Interface) error {
	if err := handler.SetLogger(independent.Logger); err != nil {
		return fmt.Errorf("handler(id: '%s').SetLogger: %w", handler.Config().Id, err)
	}

	if err := handler.Start(); err != nil {
		return fmt.Errorf("handler(category: '%s').Start: %w", handler.Config().Category, err)
	}

	return nil
}

func (independent *Independent) startHandlers() error {
	var err error
	startedAmount := 0

	for category, raw := range independent.Handlers {
		handler := raw.(base.Interface)
		if handler.Config() == nil {
			return fmt.Errorf("handler of %s category not set, please call SetConfig of handler", category)
		}
		if err = independent.setHandlerClient(handler); err != nil {
			err = fmt.Errorf("setHandlerClient('%s'): %w", category, err)
			goto exitStartHandler
		}

		if err = independent.startHandler(handler); err != nil {
			err = fmt.Errorf("startHandler: %w", err)
			goto exitStartHandler
		}
		startedAmount++
	}

exitStartHandler:
	if err == nil {
		return nil
	}

	if startedAmount == 0 {
		return err
	}
	return independent.closeHandlers(startedAmount)
}

func (independent *Independent) closeHandlers(startedAmount int) error {
	var err error

	if startedAmount == 0 {
		return err
	}

	for category, raw := range independent.Handlers {
		handler := raw.(base.Interface)
		handlerClient, newErr := manager_client.New(handler.Config())

		if newErr != nil {
			return fmt.Errorf("%v: manager_client.New('%s'): %w", err, category, newErr)
		} else {
			if closeErr := handlerClient.Close(); closeErr != nil {
				return fmt.Errorf("%v: handlerClient('%s').Close: %w", err, category, closeErr)
			}
		}

		startedAmount--
		if startedAmount == 0 {
			break
		}
	}

	return nil
}

// Start the service.
//
// Requires at least one handler.
func (independent *Independent) Start() (*sync.WaitGroup, error) {
	var err error

	if len(independent.Handlers) == 0 {
		err = fmt.Errorf("no Handlers. call service.SetHandler")
		goto errOccurred
	}

	if err = independent.setConfig(); err != nil {
		err = fmt.Errorf("setConfig: %w", err)
		goto errOccurred
	}

	independent.runtimeHandler.SetService(independent.name, independent.name)
	if !independent.runtimeHandler.IsDepManagerRunning() {
		if err = independent.runtimeHandler.StartDepManager(); err != nil {
			err = fmt.Errorf("runtimeHandler.StartDepManager: %w", err)
			goto errOccurred
		}
	}
	if !independent.runtimeHandler.IsProxyHandlerRunning() {
		if err = independent.runtimeHandler.StartProxyHandler(); err != nil {
			err = fmt.Errorf("runtimeHandler.StartProxyHandler: %w", err)
			goto errOccurred
		}
	}

	if err = independent.newManager(); err != nil {
		err = fmt.Errorf("newManager: %w", err)
		goto errOccurred
	}

	// get the proxies from the proxy chain for this service.
	// must be called before starting handlers, as routing of the handlers maybe set by proxy units.
	if err = independent.setProxyUnits(); err != nil {
		err = fmt.Errorf("independent.setProxyUnits: %w", err)
		goto errOccurred
	}

	err = independent.startHandlers()
	if err != nil {
		goto errOccurred
	}

	// todo prepare the extensions by calling them in the context.
	// todo prepare the extensions by setting them into the independent.manager.

	if err = independent.manager.Start(); err != nil {
		err = fmt.Errorf("service.manager.Start: %w", err)
		goto errOccurred
	}

	// todo add a manager command that reads the client configuration status GENERATED
	// todo upon reading it sets it into the independent.Config.Sources
	if err = independent.runtimeHandler.ProxyClient().StartLastProxies(); err != nil {
		err = fmt.Errorf("runtimeHandler.ProxyClient.StartLastProxies: %w", err)
		goto errOccurred
	}

	//err = independent.Context.ServiceReady(independent.Logger)
	//if err != nil {
	//	goto errOccurred
	//}

errOccurred:
	if err != nil {
		closeErr := independent.runtimeHandler.Close()
		if closeErr != nil {
			err = fmt.Errorf("%v: runtimeHandler.Close: %w", err, closeErr)
		}

		if independent.manager != nil && independent.manager.Running() {
			closeErr := independent.manager.Close()
			if closeErr != nil {
				err = fmt.Errorf("%v: manager.Close: %w", err, closeErr)
			}
		}
	}

	if err == nil {
		independent.blocker = &sync.WaitGroup{}
		independent.blocker.Add(1)
	}

	return independent.blocker, err
}

//func (independent *Independent) prepareExtensionConfiguration(dep *dev.Dep) error {
//	err := dep.Prepare(independent.Logger)
//	if err != nil {
//		return fmt.Errorf("dev.Prepare(%s): %w", dep.Url(), err)
//	}
//
//	err = dep.PrepareConfig(independent.Logger)
//	if err != nil {
//		return fmt.Errorf("dev.PrepareConfig on %s: %w", dep.Url(), err)
//	}
//
//	//depConfig, err := dep.GetServiceConfig()
//	//converted, err := converter.ServiceToExtension(depConfig)
//	//if err != nil {
//	//	return fmt.Errorf("config.ServiceToExtension: %w", err)
//	//}
//	//
//	//extensionConfiguration := independent.config.GetExtension(dep.Url())
//	//if extensionConfiguration == nil {
//	//	independent.config.SetExtension(&converted)
//	//} else {
//	//	if strings.Compare(extensionConfiguration.Url, converted.Url) != 0 {
//	//		return fmt.Errorf("the extension url in your '%s' config not matches to '%s' in the dependency", extensionConfiguration.Url, converted.Url)
//	//	}
//	//	if extensionConfiguration.Port != converted.Port {
//	//		independent.Logger.Warn("dependency port not matches to the extension port. Overwriting the source", "port", extensionConfiguration.Port, "dependency port", converted.Port)
//	//
//	//		main, _ := depConfig.GetFirstController()
//	//		main.Instances[0].Port = extensionConfiguration.Port
//	//
//	//depConfig.SetController(main)
//	//
//	//err = dep.SetServiceConfig(depConfig)
//	//if err != nil {
//	//	return fmt.Errorf("failed to update port in dependency extension: '%s': %w", dep.Url(), err)
//	//}
//	//}
//	//}
//
//	return nil
//}
