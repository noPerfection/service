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

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/log"
	"github.com/noPerfection/protocol/handler/base"
	"github.com/noPerfection/protocol/handler/manager_client"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service/manager"
	"github.com/noPerfection/topology"
	serviceConfig "github.com/noPerfection/topology/config/service"
)

const DefaultName = "main"
const DefaultRuntimeEndpoint = message.NewEndpoint("main_runtime", 0)
const DefaultConfigPath = "noPerfection.json"

// Independent keeps all necessary parameters of the independent service.
type Independent struct {
	topologyHandler    *topology.Handler // topology handles the configuration and dependencies
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
// Optional parameters are name, topology config path, and topology endpoint.
//
// It will also create the context internally and start it.
func New(params ...interface{}) (*Independent, error) {
	name := DefaultName
	configPath := DefaultConfigPath
	topologyEndpoint := DefaultRuntimeEndpoint

	if len(params) > 3 {
		return nil, fmt.Errorf("too many arguments, expected name, config path, and topology endpoint")
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
			return nil, fmt.Errorf("topology endpoint argument must be message.Endpoint")
		}
		topologyEndpoint = endpointArg
	}

	// Start the topology handler.
	topologyHandler, err := topology.NewHandler(configPath, topologyEndpoint)
	if err != nil {
		return nil, fmt.Errorf("topology.NewHandler: %w", err)
	}

	independent := &Independent{
		topologyHandler: topologyHandler,
		Handlers:        datatype.New(),
		name:            name,
		blocker:         nil,
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

// Name returns the unique name of the service
func (independent *Independent) Name() string {
	return independent.name
}

// Type returns the configuration type for an independent service.
func (independent *Independent) Type() serviceConfig.Type {
	return serviceConfig.IndependentType
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

// newManager creates a manager.Manager and assigns it to manager, otherwise manager is nil.
//
// Service configuration is defined in topology/config.
//
// The manager.Manager depends on Logger, set automatically.
//
// This function lints manager.Manager with the topology handler.
func (independent *Independent) newManager() error {
	m, err := manager.New(independent.topologyHandler, independent.name, &independent.blocker)
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
			return fmt.Errorf("handler of %s category has no config", category)
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

	if !independent.topologyHandler.IsDepManagerRunning() {
		if err = independent.topologyHandler.StartDepManager(); err != nil {
			err = fmt.Errorf("topologyHandler.StartDepManager: %w", err)
			goto errOccurred
		}
	}
	if !independent.topologyHandler.IsProxyHandlerRunning() {
		if err = independent.topologyHandler.StartProxyHandler(); err != nil {
			err = fmt.Errorf("topologyHandler.StartProxyHandler: %w", err)
			goto errOccurred
		}
	}

	if err = independent.newManager(); err != nil {
		err = fmt.Errorf("newManager: %w", err)
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

	//err = independent.Context.ServiceReady(independent.Logger)
	//if err != nil {
	//	goto errOccurred
	//}

errOccurred:
	if err != nil {
		closeErr := independent.topologyHandler.Close()
		if closeErr != nil {
			err = fmt.Errorf("%v: topologyHandler.Close: %w", err, closeErr)
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
