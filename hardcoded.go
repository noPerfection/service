package service

import (
	"fmt"

	"github.com/noPerfection/service/handlers"
	"github.com/noPerfection/topology/config"
)

// WithHardcodedTopology keeps handler topology configs set from code.
type WithHardcodedTopology struct {
	name string
	// service name -> service config
	serviceConfigs map[string]config.Service
	// service name -> handler configs
	handlerConfigs map[string][]config.Handler
	// service name -> deps
	handlerDeps map[string][]config.DepService
	// service name -> handler category -> deps
	commandDeps map[string]map[string][]config.DepService
}

// NewHardcodedTopologies creates storage for code-defined topology configs.
func NewHardcodedTopologies(serviceName string) *WithHardcodedTopology {
	if serviceName == "" {
		serviceName = DefaultName
	}

	return &WithHardcodedTopology{
		name:           serviceName,
		serviceConfigs: make(map[string]config.Service),
		handlerConfigs: make(map[string][]config.Handler),
		handlerDeps:    make(map[string][]config.DepService),
		commandDeps:    make(map[string]map[string][]config.DepService),
	}
}

// SetServiceConfig stores a service config to be written into topology.
func (topologies *WithHardcodedTopology) SetServiceConfig(service config.Service) error {
	if topologies == nil {
		return fmt.Errorf("hardcoded topologies is nil")
	}
	if service.Name == "" {
		service.Name = topologies.name
	}
	if service.Name == "" {
		service.Name = DefaultName
	}

	topologies.serviceConfigs[service.Name] = service
	return nil
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

// SetCommandDeps stores command dependencies by service and handler category.
func (topologies *WithHardcodedTopology) SetCommandDeps(dep config.DepService, handlerAndServiceName ...string) error {
	if topologies == nil {
		return fmt.Errorf("hardcoded topologies is nil")
	}
	if len(handlerAndServiceName) > 2 {
		return fmt.Errorf("too many arguments, expected handler category and service name")
	}
	if dep.Name == "" {
		return fmt.Errorf("dep service name is empty")
	}

	handlerCategory := handlers.DefaultHandlerCategory
	if len(handlerAndServiceName) > 0 && handlerAndServiceName[0] != "" {
		handlerCategory = handlerAndServiceName[0]
	}

	serviceName := topologies.name
	if len(handlerAndServiceName) > 1 && handlerAndServiceName[1] != "" {
		serviceName = handlerAndServiceName[1]
	}
	if serviceName == "" {
		serviceName = DefaultName
	}

	if topologies.commandDeps[serviceName] == nil {
		topologies.commandDeps[serviceName] = make(map[string][]config.DepService)
	}

	deps := topologies.commandDeps[serviceName][handlerCategory]
	for i := range deps {
		if deps[i].Name == dep.Name {
			deps[i] = dep
			topologies.commandDeps[serviceName][handlerCategory] = deps
			return nil
		}
	}

	topologies.commandDeps[serviceName][handlerCategory] = append(deps, dep)
	return nil
}

// SetHandlerDeps stores handler dependencies by service.
func (topologies *WithHardcodedTopology) SetHandlerDeps(dep config.DepService, serviceName ...string) error {
	if topologies == nil {
		return fmt.Errorf("hardcoded topologies is nil")
	}
	if len(serviceName) > 1 {
		return fmt.Errorf("too many service names")
	}
	if dep.Name == "" {
		return fmt.Errorf("dep service name is empty")
	}

	name := topologies.name
	if len(serviceName) == 1 && serviceName[0] != "" {
		name = serviceName[0]
	}
	if name == "" {
		name = DefaultName
	}

	topologies.handlerDeps[name] = setDepService(topologies.handlerDeps[name], dep)
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

func (independent *Independent) addHardcodedServicesToTopology() error {
	if independent == nil || independent.WithHardcodedTopology == nil {
		return fmt.Errorf("service or WithHardcodedTopology is nil")
	}

	for serviceName, serviceConfig := range independent.serviceConfigs {
		// If the service is not in the topology, add it.
		_, err := independent.topologyHandler.Service(serviceName)
		if err != nil {
			if err := independent.topologyHandler.AddService(serviceConfig); err != nil {
				if err := independent.topologyHandler.SetService(serviceConfig); err != nil {
					return fmt.Errorf("topologyHandler.SetService('%s'): %w", serviceName, err)
				}
			}
			continue
		}

		// Otherwise, update it.
		if err := independent.topologyHandler.SetService(serviceConfig); err != nil {
			return fmt.Errorf("topologyHandler.SetService('%s'): %w", serviceName, err)
		}
	}

	return nil
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
			serviceConfig.SetHandler(config.NewHandlerVariant(handler), true)
		}
		if err := independent.topologyHandler.SetService(serviceConfig); err != nil {
			return fmt.Errorf("topologyHandler.SetService('%s'): %w", serviceName, err)
		}
	}

	return nil
}

func (independent *Independent) addHardcodedHandlerDepsToTopology() error {
	if independent == nil || independent.WithHardcodedTopology == nil {
		return fmt.Errorf("service or WithHardcodedTopology is nil")
	}

	for serviceName, deps := range independent.handlerDeps {
		serviceConfig, err := independent.topologyHandler.Service(serviceName)
		if err != nil {
			return fmt.Errorf("hardcoded handler deps for '%s' service not found in topology: %w", serviceName, err)
		}

		for _, dep := range deps {
			serviceConfig.HandlerDeps = setDepService(serviceConfig.HandlerDeps, dep)
		}
		if err := independent.topologyHandler.SetService(serviceConfig); err != nil {
			return fmt.Errorf("topologyHandler.SetService('%s'): %w", serviceName, err)
		}
	}

	return nil
}

func (independent *Independent) addHardcodedCommandDepsToTopology() error {
	if independent == nil || independent.WithHardcodedTopology == nil {
		return fmt.Errorf("service or WithHardcodedTopology is nil")
	}

	for serviceName, depsByHandler := range independent.commandDeps {
		serviceConfig, err := independent.topologyHandler.Service(serviceName)
		if err != nil {
			return fmt.Errorf("hardcoded command deps for '%s' service not found in topology: %w", serviceName, err)
		}

		for handlerCategory, deps := range depsByHandler {
			handlerVariant, err := serviceConfig.HandlerByCategory(handlerCategory)
			if err != nil {
				return fmt.Errorf("hardcoded command deps handler '%s' in service '%s': %w", handlerCategory, serviceName, err)
			}

			for _, dep := range deps {
				setHandlerVariantCommandDep(&handlerVariant, dep)
			}
			serviceConfig.SetHandler(handlerVariant, true)
		}
		if err := independent.topologyHandler.SetService(serviceConfig); err != nil {
			return fmt.Errorf("topologyHandler.SetService('%s'): %w", serviceName, err)
		}
	}

	return nil
}

func setHandlerVariantCommandDep(handler *config.HandlerVariant, dep config.DepService) {
	if handler.ProxyHandler != nil {
		handler.ProxyHandler.CommandDeps = setDepService(handler.ProxyHandler.CommandDeps, dep)
		return
	}
	if handler.Handler != nil {
		handler.Handler.CommandDeps = setDepService(handler.Handler.CommandDeps, dep)
	}
}

func setDepService(deps []config.DepService, dep config.DepService) []config.DepService {
	for i := range deps {
		if deps[i].Name == dep.Name {
			deps[i] = dep
			return deps
		}
	}

	return append(deps, dep)
}
