package handlers

import (
	"fmt"
	"sort"

	"github.com/noPerfection/log"
	clientSyncReplier "github.com/noPerfection/protocol/client/sync_replier"
	"github.com/noPerfection/protocol/handler/base"
	handlerControl "github.com/noPerfection/protocol/handler/control"
	"github.com/noPerfection/protocol/message"
)

const DefaultHandlerCategory = "main"

var DefaultHandlerEndpoint = message.NewEndpoint("localhost", 8000)

// Handlers owns the local handler registry and lifecycle.
type Handlers struct {
	// handler category -> handler/base.Interface
	handlers map[string]base.Interface
	// handler category -> command -> handle function
	routes  map[string]map[string]base.HandleFunc
	logger  *log.Logger
	running bool
}

// NewHandlers creates an empty handler manager.
func NewHandlers() *Handlers {
	return &Handlers{
		handlers: make(map[string]base.Interface),
		routes:   make(map[string]map[string]base.HandleFunc),
	}
}

// SetHandler adds or replaces a handler by category.
func (manager *Handlers) SetHandler(category string, handler base.Interface) error {
	if handler == nil {
		return fmt.Errorf("handler of %s category is nil", category)
	}
	if _, exists := manager.handlers[category]; exists {
		return fmt.Errorf("handler of %s category already exists", category)
	}
	manager.handlers[category] = handler

	return nil
}

func (manager *Handlers) IsHandlerExist(category string) bool {
	_, exists := manager.handlers[category]
	return exists
}

func (manager *Handlers) RouteCommands(category string) ([]string, error) {
	handler, exists := manager.handlers[category]
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

	for category, handler := range manager.handlers {
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

	for category, handler := range manager.handlers {
		if handler == nil {
			err = fmt.Errorf("handler of %s category is nil", category)
			goto exitStartHandler
		}
		if handler.Endpoint() == (message.Endpoint{}) {
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
	for category, handler := range manager.handlers {
		if handler == nil {
			return fmt.Errorf("handler of %s category is nil", category)
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
		if err := closeHandlerViaControl(handler); err != nil {
			return err
		}
	}

	return nil
}

func newHandlerControlClient(handler base.Interface) (*clientSyncReplier.BaseControl, error) {
	endpoint := handler.Endpoint()
	if endpoint == (message.Endpoint{}) {
		return nil, fmt.Errorf("handler endpoint is empty")
	}

	controlEndpoint := handlerControl.NewInternalControlEndpoint(endpoint)
	controlClient, err := clientSyncReplier.NewBaseControl(controlEndpoint.Id, controlEndpoint.Port)
	if err != nil {
		return nil, fmt.Errorf("sync_replier.NewBaseControl('%s'): %w", controlEndpoint.Id, err)
	}

	return controlClient, nil
}

func closeHandlerViaControl(handler base.Interface) error {
	controlClient, err := newHandlerControlClient(handler)
	if err != nil {
		return nil
	}
	defer func() {
		_ = controlClient.Close()
	}()

	status, err := controlClient.HandlerStatus()
	if err != nil {
		return nil
	}
	if status == base.SocketNil {
		return nil
	}

	if err := controlClient.HandlerClose(); err != nil {
		return fmt.Errorf("handler(endpoint: '%s').HandlerClose: %w", handler.Endpoint().Id, err)
	}

	return nil
}
