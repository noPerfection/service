package handlers

import (
	"fmt"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/log"
	"github.com/noPerfection/protocol/handler/base"
	"github.com/noPerfection/protocol/message"
)

const DefaultHandlerCategory = "main"

var DefaultHandlerEndpoint = message.NewEndpoint("localhost", 8000)

// Handlers owns the local handler registry and lifecycle.
type Handlers struct {
	handlers datatype.KeyValue
	routes   map[string]map[string]base.HandleFunc
	logger   *log.Logger
	running  bool
}

// NewHandlers creates an empty handler manager.
func NewHandlers() *Handlers {
	return &Handlers{
		handlers: datatype.New(),
		routes:   make(map[string]map[string]base.HandleFunc),
	}
}

// SetHandler adds or replaces a handler by category.
func (manager *Handlers) SetHandler(category string, handler base.Interface) error {
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

func (manager *Handlers) IsHandlerExist(category string) bool {
	_, exists := manager.handlers[category]
	return exists
}

func (manager *Handlers) Route(command string, handleFunc base.HandleFunc, handlerCategory ...string) error {
	if manager.running {
		return fmt.Errorf("I cant route when its already started. Please stop the handler first or the best way to route before starting the handler")
	}
	if len(handlerCategory) > 1 {
		return fmt.Errorf("too many handler categories")
	}

	category := DefaultHandlerCategory
	if len(handlerCategory) == 1 && handlerCategory[0] != "" {
		category = handlerCategory[0]
	}
	if manager.routes[category] == nil {
		manager.routes[category] = make(map[string]base.HandleFunc)
	}
	manager.routes[category][command] = handleFunc

	return nil
}

// SetLogger sets the optional logger for this manager and all registered handlers.
func (manager *Handlers) SetLogger(logger *log.Logger) error {
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
// The manager itself is not a thread to run
func (manager *Handlers) Start() error {
	var err error
	startedHandlers := make([]base.Interface, 0, len(manager.handlers))

	if len(manager.handlers) == 0 {
		return fmt.Errorf("no handlers")
	}
	for category := range manager.routes {
		if !manager.IsHandlerExist(category) {
			return fmt.Errorf("routed to a category that not exist: '%s'", category)
		}
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

		for command, handleFunc := range manager.routes[category] {
			if err = handler.Route(command, handleFunc); err != nil {
				err = fmt.Errorf("handler(category: '%s').Route('%s'): %w", category, command, err)
				goto exitStartHandler
			}
		}

		if err = handler.Start(); err != nil {
			err = fmt.Errorf("handler(category: '%s').Start: %w", category, err)
			goto exitStartHandler
		}
		startedHandlers = append(startedHandlers, handler)
	}
	manager.running = true
	manager.routes = nil

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
func (manager *Handlers) Close() error {
	handlers := make([]base.Interface, 0, len(manager.handlers))
	for category, raw := range manager.handlers {
		handler, ok := raw.(base.Interface)
		if !ok {
			return fmt.Errorf("handler of %s category is not a base.Interface", category)
		}
		handlers = append(handlers, handler)
	}

	if err := closeHandlers(handlers); err != nil {
		return err
	}
	manager.routes = make(map[string]map[string]base.HandleFunc)
	manager.running = false

	return nil
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
