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
	"github.com/noPerfection/service/manager"
	"github.com/noPerfection/topology"
	"github.com/noPerfection/topology/config"
)

const DefaultName = "main"
const DefaultConfigPath = "noPerfection.json"

// Independent keeps all necessary parameters of the independent service.
type Independent struct {
	topologyHandler *topology.Handler // topology handles the configuration and dependencies
	Handlers        datatype.KeyValue
	Logger          *log.Logger
	name            string
	// The blocker is a shutdown latch:
	// it keeps the process alive after Start() returns, and it unblocks when the service is fully closed.
	blocker *sync.WaitGroup
	manager *manager.Manager // manage this service from other parts
}

// Return instance of an independent service.
// Optional parameters are name and topology config path.
func New(params ...interface{}) (*Independent, error) {
	name := DefaultName
	configPath := DefaultConfigPath

	if len(params) > 2 {
		return nil, fmt.Errorf("too many arguments, expected name and config path")
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

	m, err := manager.New(topologyHandler, name, nil)
	if err != nil {
		return nil, fmt.Errorf("manager.New: %w", err)
	}

	independent := &Independent{
		topologyHandler: topologyHandler,
		Handlers:        datatype.New(),
		name:            name,
		manager:         m,
		blocker:         nil,
	}

	return independent, nil
}

// EnableLogger toggles the optional service logger.
func (independent *Independent) EnableLogger(enable bool) error {
	if !enable {
		independent.Logger = nil
		return nil
	}

	logger, err := log.New(independent.name, true)
	if err != nil {
		return fmt.Errorf("log.New(%s): %w", independent.name, err)
	}
	independent.Logger = logger

	if independent.manager != nil {
		if err := independent.manager.SetLogger(logger); err != nil {
			return fmt.Errorf("manager.SetLogger: %w", err)
		}
	}

	return nil
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
func (independent *Independent) Type() config.Type {
	return config.IndependentType
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
	if independent.Logger != nil {
		if err := handler.SetLogger(independent.Logger); err != nil {
			return fmt.Errorf("handler(id: '%s').SetLogger: %w", handler.Config().Category, err)
		}
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

	if err = independent.topologyHandler.Start(); err != nil {
		err = fmt.Errorf("topologyHandler.Start(): %w", err)
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
