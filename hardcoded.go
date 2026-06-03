package service

import (
	"fmt"

	"github.com/noPerfection/topology/config"
)

// WithHardcodedTopology keeps handler topology configs set from code.
type WithHardcodedTopology struct {
	name           string
	handlerConfigs map[string][]config.Handler
}

// NewHardcodedTopologies creates storage for code-defined topology configs.
func NewHardcodedTopologies(serviceName string) *WithHardcodedTopology {
	if serviceName == "" {
		serviceName = DefaultName
	}

	return &WithHardcodedTopology{
		name:           serviceName,
		handlerConfigs: make(map[string][]config.Handler),
	}
}

// SetHandlerConfig stores a handler config by category for the given service.
func (topologies *WithHardcodedTopology) SetHandlerConfig(handler config.Handler, serviceName ...string) error {
	if topologies == nil {
		return fmt.Errorf("hardcoded topologies is nil")
	}
	if len(serviceName) > 1 {
		return fmt.Errorf("too many service names")
	}
	if handler.Category == "" {
		return fmt.Errorf("handler category is empty")
	}

	name := topologies.name
	if len(serviceName) == 1 && serviceName[0] != "" {
		name = serviceName[0]
	}
	if name == "" {
		name = topologies.name
	}

	handlers := topologies.handlerConfigs[name]
	for i := range handlers {
		if handlers[i].Category == handler.Category {
			handlers[i] = handler
			topologies.handlerConfigs[name] = handlers
			return nil
		}
	}

	topologies.handlerConfigs[name] = append(handlers, handler)
	return nil
}

func (topologies *WithHardcodedTopology) HasHardcodedHandlers(serviceName ...string) bool {
	if topologies == nil {
		return false
	}

	name := topologies.name
	if len(serviceName) > 0 && serviceName[0] != "" {
		name = serviceName[0]
	}
	return len(topologies.handlerConfigs[name]) > 0
}

func (independent *Independent) addHardcodedHandlersToTopology() error {
	if independent == nil || independent.WithHardcodedTopology == nil {
		return fmt.Errorf("service or WithHardcodedTopology is nil")
	}

	for serviceName, handlers := range independent.handlerConfigs {
		serviceConfig, err := independent.topologyHandler.Service(serviceName)
		if err != nil {
			return fmt.Errorf("hardcoded handlers for '%s' service not found in topology: %w", serviceName, err)
		}

		for _, handler := range handlers {
			serviceConfig.SetHandler(handler, true)
		}
		if err := independent.topologyHandler.SetService(serviceConfig); err != nil {
			return fmt.Errorf("topologyHandler.SetService('%s'): %w", serviceName, err)
		}
	}

	return nil
}
