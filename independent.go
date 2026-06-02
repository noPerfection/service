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

	"github.com/noPerfection/log"
	"github.com/noPerfection/service/handlers"
	"github.com/noPerfection/service/manager"
	"github.com/noPerfection/topology"
	"github.com/noPerfection/topology/config"
)

const DefaultName = "main"
const DefaultConfigPath = "noPerfection.json"

// Independent keeps all necessary parameters of the independent service.
type Independent struct {
	*handlers.Manager
	topologyHandler *topology.Handler // topology handles the configuration and dependencies
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
		Manager:         handlers.NewManager(),
		topologyHandler: topologyHandler,
		name:            name,
		manager:         m,
		blocker:         nil,
	}

	return independent, nil
}

// EnableLogger toggles the optional service logger.
func (independent *Independent) EnableLogger(enable bool) error {
	if !enable {
		if err := independent.Manager.SetLogger(nil); err != nil {
			return fmt.Errorf("handlers.SetLogger: %w", err)
		}
		return nil
	}

	logger, err := log.New(independent.name, true)
	if err != nil {
		return fmt.Errorf("log.New(%s): %w", independent.name, err)
	}
	if err := independent.Manager.SetLogger(logger); err != nil {
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

// Start the service.
//
// Requires at least one handler.
func (independent *Independent) Start() (*sync.WaitGroup, error) {
	var err error

	if err = independent.topologyHandler.Start(); err != nil {
		err = fmt.Errorf("topologyHandler.Start(): %w", err)
		goto errOccurred
	}

	if err = independent.Manager.Start(); err != nil {
		err = fmt.Errorf("handlers.Start: %w", err)
		goto errOccurred
	}

	// todo prepare the extensions by calling them in the context.
	// todo prepare the extensions by setting them into the independent.manager.

	if err = independent.manager.Start(); err != nil {
		err = fmt.Errorf("service.manager.Start: %w", err)
		goto errOccurred
	}

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
