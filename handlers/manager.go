package handlers

import (
	"fmt"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/log"
	"github.com/noPerfection/protocol/handler/base"
)

// Manager owns the local handler registry and lifecycle.
type Manager struct {
	handlers datatype.KeyValue
	logger   *log.Logger
}

// NewManager creates an empty handler manager.
func NewManager() *Manager {
	return &Manager{
		handlers: datatype.New(),
	}
}

// SetHandler adds or replaces a handler by category.
func (manager *Manager) SetHandler(category string, handler base.Interface) error {
	if raw, exists := manager.handlers[category]; exists {
		registered, ok := raw.(base.Interface)
		if !ok {
			return fmt.Errorf("handler of %s category is not a base.Interface", category)
		}
		if !registered.Closed() {
			if err := closeHandlers([]base.Interface{registered}); err != nil {
				return fmt.Errorf("close existing handler(category: '%s'): %w", category, err)
			}
		}
	}
	manager.handlers.Set(category, handler)

	return nil
}

// SetLogger sets the optional logger for this manager and all registered handlers.
func (manager *Manager) SetLogger(logger *log.Logger) error {
	manager.logger = logger

	for category, raw := range manager.handlers {
		handler, ok := raw.(base.Interface)
		if !ok {
			return fmt.Errorf("handler of %s category is not a base.Interface", category)
		}
		if err := handler.SetLogger(logger); err != nil {
			return fmt.Errorf("handler(category: '%s').SetLogger: %w", category, err)
		}
	}

	return nil
}

// Start starts all registered handlers.
func (manager *Manager) Start() error {
	var err error
	startedHandlers := make([]base.Interface, 0, len(manager.handlers))

	if len(manager.handlers) == 0 {
		return fmt.Errorf("no handlers")
	}

	for category, raw := range manager.handlers {
		handler, ok := raw.(base.Interface)
		if !ok {
			err = fmt.Errorf("handler of %s category is not a base.Interface", category)
			goto exitStartHandler
		}
		if handler.Config() == nil {
			err = fmt.Errorf("handler of %s category has no config", category)
			goto exitStartHandler
		}

		if manager.logger != nil {
			if err = handler.SetLogger(manager.logger); err != nil {
				err = fmt.Errorf("handler(category: '%s').SetLogger: %w", category, err)
				goto exitStartHandler
			}
		}

		if err = handler.Start(); err != nil {
			err = fmt.Errorf("handler(category: '%s').Start: %w", category, err)
			goto exitStartHandler
		}
		startedHandlers = append(startedHandlers, handler)
	}

exitStartHandler:
	if err == nil {
		return nil
	}

	if len(startedHandlers) == 0 {
		return err
	}
	if closeErr := closeHandlers(startedHandlers); closeErr != nil {
		return fmt.Errorf("%v: close started handlers: %w", err, closeErr)
	}

	return err
}

// Close closes all registered handlers.
// Used only by service codes during the start-ups.
// After the service is started, the handlers are closed by the service/manager
func (manager *Manager) Close() error {
	handlers := make([]base.Interface, 0, len(manager.handlers))
	for category, raw := range manager.handlers {
		handler, ok := raw.(base.Interface)
		if !ok {
			return fmt.Errorf("handler of %s category is not a base.Interface", category)
		}
		handlers = append(handlers, handler)
	}

	return closeHandlers(handlers)
}

func closeHandlers(handlers []base.Interface) error {
	for _, handler := range handlers {
		handler.SetClose(true)
		if socket := handler.Socket(); socket != nil {
			if err := socket.Close(); err != nil {
				return fmt.Errorf("handler(category: '%s').Socket.Close: %w", handler.Config().Category, err)
			}
			handler.SetSocketNil()
		}
	}

	return nil
}
