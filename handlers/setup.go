package handlers

import (
	"fmt"
	"sort"
	"time"

	"github.com/noPerfection/log"
	protocolClient "github.com/noPerfection/protocol/client"
	protocolHandler "github.com/noPerfection/protocol/handler"
	"github.com/noPerfection/protocol/message"
)

const DefaultHandlerCategory = "main"

var DefaultHandlerEndpoint = message.NewEndpoint("localhost", 8000)

// Setup owns the local handler registry and lifecycle.
type Setup struct {
	// handler category -> handler.Interface
	handlers map[string]protocolHandler.Interface
	// handler category -> command -> handle function
	routes  map[string]map[string]protocolHandler.HandleFunc
	logger  *log.Logger
	running bool
}

// NewSetup creates an empty handler manager.
func NewSetup() *Setup {
	return &Setup{
		handlers: make(map[string]protocolHandler.Interface),
		routes:   make(map[string]map[string]protocolHandler.HandleFunc),
	}
}

// SetHandler adds or replaces a handler by category.
func (setup *Setup) SetHandler(category string, handler protocolHandler.Interface) error {
	if handler == nil {
		return fmt.Errorf("handler of %s category is nil", category)
	}
	if _, exists := setup.handlers[category]; exists {
		return fmt.Errorf("handler of %s category already exists", category)
	}
	setup.handlers[category] = handler

	return nil
}

func (setup *Setup) IsHandlerExist(category string) bool {
	_, exists := setup.handlers[category]
	return exists
}

func (setup *Setup) RouteCommands(category string) ([]string, error) {
	handler, exists := setup.handlers[category]
	if !exists {
		return nil, fmt.Errorf("handler of %s category is not found", category)
	}
	if handler == nil {
		return nil, fmt.Errorf("handler of %s category is nil", category)
	}

	commands := handler.Commands()
	sort.Strings(commands)
	return commands, nil
}

func (setup *Setup) Route(command string, handleFunc protocolHandler.HandleFunc, handlerCategory ...string) error {
	if setup.running {
		return fmt.Errorf("I cant route when its already started. Please stop the handler first or the best way to route before starting the handler")
	}
	if len(handlerCategory) > 1 {
		return fmt.Errorf("too many handler categories")
	}

	category := DefaultHandlerCategory
	if len(handlerCategory) == 1 && handlerCategory[0] != "" {
		category = handlerCategory[0]
	}
	if setup.routes[category] == nil {
		setup.routes[category] = make(map[string]protocolHandler.HandleFunc)
	}
	setup.routes[category][command] = handleFunc

	return nil
}

// SetLogger sets the optional logger for this setup and all registered handlers.
func (setup *Setup) SetLogger(logger *log.Logger) error {
	setup.logger = logger

	for category, handler := range setup.handlers {
		if handler == nil {
			return fmt.Errorf("handler of %s category is nil", category)
		}
		if err := handler.SetLogger(logger); err != nil {
			return fmt.Errorf("handler(category: '%s').SetLogger: %w", category, err)
		}
	}

	return nil
}

// Start starts all registered handlers.
// The setup itself is not a thread to run
func (setup *Setup) Start(serviceLink string) error {
	if len(setup.handlers) == 0 {
		return fmt.Errorf("no handlers")
	}

	for category := range setup.routes {
		if !setup.IsHandlerExist(category) {
			return fmt.Errorf("routed to a category that not exist: '%s'", category)
		}
	}

	var err error
	startedHandlers := make([]protocolHandler.Interface, 0, len(setup.handlers))

	for category, handler := range setup.handlers {
		if handler == nil {
			err = fmt.Errorf("handler of %s category is nil", category)
			goto exitStartHandler
		}
		if handler.Endpoint() == (message.Endpoint{}) {
			err = fmt.Errorf("handler of %s category has no config", category)
			goto exitStartHandler
		}

		if setup.logger != nil {
			if err = handler.SetLogger(setup.logger); err != nil {
				err = fmt.Errorf("handler(category: '%s').SetLogger: %w", category, err)
				goto exitStartHandler
			}
		}

		for command, handleFunc := range setup.routes[category] {
			if err = handler.Route(command, handleFunc); err != nil {
				err = fmt.Errorf("handler(category: '%s').Route('%s'): %w", category, command, err)
				goto exitStartHandler
			}
		}

		handlerLink, err := AsHandlerLink(serviceLink, category)
		if err != nil {
			err = fmt.Errorf("handlers.AsHandlerLink(%q): %w", category, err)
			goto exitStartHandler
		}
		handler.SetMushroomURL(handlerLink)

		if err = handler.Start(); err != nil {
			err = fmt.Errorf("handler(category: '%s').Start: %w", category, err)
			goto exitStartHandler
		}
		startedHandlers = append(startedHandlers, handler)
	}
	setup.running = true
	setup.routes = nil

exitStartHandler:
	if err == nil {
		return nil
	}

	if len(startedHandlers) == 0 {
		return err
	}
	for _, handler := range startedHandlers {
		if err := CloseViaControl(handler); err != nil {
			return fmt.Errorf("%v: close started handlers: %w", err, err)
		}
	}

	return err
}

// Close closes all registered handlers.
// Used only by service codes during the start-ups.
// After the service is started, the handlers are closed by the service/setup
func (setup *Setup) Close() error {
	handlers := make([]protocolHandler.Interface, 0, len(setup.handlers))
	for category, handler := range setup.handlers {
		if handler == nil {
			return fmt.Errorf("handler of %s category is nil", category)
		}
		handlers = append(handlers, handler)
	}

	for _, handler := range handlers {
		if err := CloseViaControl(handler); err != nil {
			return err
		}
	}
	setup.routes = make(map[string]map[string]protocolHandler.HandleFunc)
	setup.running = false

	return nil
}

// CloseViaControl shuts down a handler through its control endpoint.
func CloseViaControl(handler protocolHandler.Interface) error {
	controlClient, err := newHandlerControlClient(handler)
	if err != nil {
		return nil
	}
	defer controlClient.Close()

	status, err := controlClient.HandlerStatus()
	if err != nil {
		return nil
	}
	if status == protocolHandler.SocketNil {
		return nil
	}

	if err := controlClient.HandlerClose(); err != nil {
		return fmt.Errorf("handler(endpoint: '%s').HandlerClose: %w", handler.Endpoint().Id, err)
	}

	return nil
}

func newHandlerControlClient(handler protocolHandler.Interface) (*protocolClient.Control, error) {
	endpoint := handler.Endpoint()
	if endpoint == (message.Endpoint{}) {
		return nil, fmt.Errorf("handler endpoint is empty")
	}

	controlEndpoint := protocolHandler.NewInternalControlEndpoint(endpoint)
	controlClient, err := protocolClient.NewControl(controlEndpoint.Id, controlEndpoint.Port)
	if err != nil {
		return nil, fmt.Errorf("client.NewControl('%s'): %w", controlEndpoint.Id, err)
	}
	controlClient.Timeout(time.Second)
	controlClient.Attempt(1)

	return controlClient, nil
}
