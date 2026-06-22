package service

import (
	"fmt"

	"github.com/ahmetson/mushroom"
	"github.com/noPerfection/service/handlers"
	"github.com/noPerfection/service/manager"
	"github.com/noPerfection/topology/config"
)

// WithHardcodedTopology keeps handler topology configs set from code.
type WithHardcodedTopology struct {
	// For default path
	mushroomURL string
	// mushroomURL -> service config
	serviceConfigs map[string]config.Service
	// mushroomURL -> handler configs
	handlerConfigs map[string][]config.Handler
	// mushroomURL -> deps
	handlerDeps map[string][]config.DepService
	// mushroomURL -> handler category -> deps
	commandDeps map[string]map[string][]config.DepService
}

// NewHardcodedTopologies creates storage for code-defined topology configs.
func NewHardcodedTopologies(mushroomURL string) *WithHardcodedTopology {
	return &WithHardcodedTopology{
		mushroomURL:    mushroomURL,
		serviceConfigs: make(map[string]config.Service),
		handlerConfigs: make(map[string][]config.Handler),
		handlerDeps:    make(map[string][]config.DepService),
		commandDeps:    make(map[string]map[string][]config.DepService),
	}
}

// If mushroom url is not set returns the mushroom url otherwise default one set at initiation
func (topologies *WithHardcodedTopology) resolveMushroomURL(mushroomURL ...string) (string, error) {
	if len(mushroomURL) > 1 {
		return "", fmt.Errorf("too many mushroom urls")
	}
	if len(mushroomURL) == 1 && mushroomURL[0] != "" {
		return mushroomURL[0], nil
	}
	return topologies.mushroomURL, nil
}

func serviceParentURL(mushroomURL string) []string {
	parent, ok := mushroom.ParentResourceURL(mushroomURL)
	if !ok {
		return nil
	}
	return []string{parent}
}

// SetServiceConfig stores a service config to be written into topology.
func (topologies *WithHardcodedTopology) SetServiceConfig(service config.Service, mushroomURL ...string) error {
	if topologies == nil {
		return fmt.Errorf("hardcoded topologies is nil")
	}
	url, err := topologies.resolveMushroomURL(mushroomURL...)
	if err != nil {
		return err
	}

	topologies.serviceConfigs[url] = service
	return nil
}

// SetHandlerConfig stores a handler config by category for the given service.
func (topologies *WithHardcodedTopology) SetHandlerConfig(handler config.Handler, mushroomURL ...string) error {
	if topologies == nil {
		return fmt.Errorf("hardcoded topologies is nil")
	}
	baseHandler, ok := handler.AsIndependentHandler()
	if !ok {
		return fmt.Errorf("handler is not an independent handler")
	}
	if baseHandler.Category == "" {
		return fmt.Errorf("handler category is empty")
	}

	url, err := topologies.resolveMushroomURL(mushroomURL...)
	if err != nil {
		return err
	}

	handlers := topologies.handlerConfigs[url]
	for i := range handlers {
		existing, ok := handlers[i].AsIndependentHandler()
		if ok && existing.Category == baseHandler.Category {
			handlers[i] = handler
			topologies.handlerConfigs[url] = handlers
			return nil
		}
	}

	topologies.handlerConfigs[url] = append(handlers, handler)
	return nil
}

// SetCommandDeps stores command dependencies by service and handler category.
func (topologies *WithHardcodedTopology) SetCommandDeps(dep config.DepService, handlerAndMushroomURL ...string) error {
	if topologies == nil {
		return fmt.Errorf("hardcoded topologies is nil")
	}
	if len(handlerAndMushroomURL) > 2 {
		return fmt.Errorf("too many arguments, expected handler category and mushroom url")
	}
	if dep.Name == "" {
		return fmt.Errorf("dep service name is empty")
	}

	handlerCategory := handlers.DefaultHandlerCategory
	if len(handlerAndMushroomURL) > 0 && handlerAndMushroomURL[0] != "" {
		handlerCategory = handlerAndMushroomURL[0]
	}

	url := topologies.mushroomURL
	if len(handlerAndMushroomURL) > 1 && handlerAndMushroomURL[1] != "" {
		url = handlerAndMushroomURL[1]
	}

	if topologies.commandDeps[url] == nil {
		topologies.commandDeps[url] = make(map[string][]config.DepService)
	}

	deps := topologies.commandDeps[url][handlerCategory]
	for i := range deps {
		if deps[i].Name == dep.Name {
			deps[i] = dep
			topologies.commandDeps[url][handlerCategory] = deps
			return nil
		}
	}

	topologies.commandDeps[url][handlerCategory] = append(deps, dep)
	return nil
}

// SetHandlerDeps stores handler dependencies by service.
func (topologies *WithHardcodedTopology) SetHandlerDeps(dep config.DepService, mushroomURL ...string) error {
	if topologies == nil {
		return fmt.Errorf("hardcoded topologies is nil")
	}
	if dep.Name == "" {
		return fmt.Errorf("dep service name is empty")
	}

	url, err := topologies.resolveMushroomURL(mushroomURL...)
	if err != nil {
		return err
	}

	topologies.handlerDeps[url] = setDepService(topologies.handlerDeps[url], dep)
	return nil
}

func (topologies *WithHardcodedTopology) HasHardcodedHandlers(mushroomURL ...string) bool {
	if topologies == nil {
		return false
	}

	url := topologies.mushroomURL
	if len(mushroomURL) > 0 && mushroomURL[0] != "" {
		url = mushroomURL[0]
	}
	return len(topologies.handlerConfigs[url]) > 0
}

func (independent *Independent) addHardcodedServicesToTopology() error {
	if independent == nil || independent.WithHardcodedTopology == nil {
		return fmt.Errorf("service or WithHardcodedTopology is nil")
	}

	for mushroomURL, serviceConfig := range independent.serviceConfigs {
		parent := serviceParentURL(mushroomURL)
		_, err := independent.topologyHandler.Service(mushroomURL)
		if err != nil {
			if err := independent.topologyHandler.AddService(serviceConfig, parent...); err != nil {
				if err := independent.topologyHandler.SetService(serviceConfig, parent...); err != nil {
					return fmt.Errorf("topologyHandler.SetService(%q): %w", mushroomURL, err)
				}
			}
			continue
		}

		if err := independent.topologyHandler.SetService(serviceConfig, parent...); err != nil {
			return fmt.Errorf("topologyHandler.SetService(%q): %w", mushroomURL, err)
		}
	}

	return nil
}

func (independent *Independent) addHardcodedHandlersToTopology() error {
	if independent == nil || independent.WithHardcodedTopology == nil {
		return fmt.Errorf("service or WithHardcodedTopology is nil")
	}

	for mushroomURL, handlers := range independent.handlerConfigs {
		serviceConfig, err := independent.topologyHandler.Service(mushroomURL)
		if err != nil {
			return fmt.Errorf("hardcoded handlers for %q not found in topology: %w", mushroomURL, err)
		}

		for _, handler := range handlers {
			if base, ok := handler.AsIndependentHandler(); ok && base.Category == config.ServiceManagerCategory {
				managerConfig := independent.manager.Config()
				if managerConfig.Endpoint == base.Endpoint {
					continue
				}
				m, err := manager.New(mushroomURL, base.Endpoint)
				if err != nil {
					return fmt.Errorf("manager.New: %w", err)
				}
				independent.manager = m
				if err := independent.manager.SetLogger(independent.logger); err != nil {
					return fmt.Errorf("manager.SetLogger: %w", err)
				}
				continue
			}
			if serviceConfig.Type == config.ProxyType {
				proxyHandler, ok := handler.AsProxyHandler()
				if !ok {
					continue
				}
				handler = normalizeProxyHandlerOutbounds(proxyHandler)
			}
			serviceConfig.SetHandler(handler, true)
		}
		if err := independent.topologyHandler.SetService(serviceConfig, serviceParentURL(mushroomURL)...); err != nil {
			return fmt.Errorf("topologyHandler.SetService(%q): %w", mushroomURL, err)
		}
	}

	return nil
}

func (independent *Independent) addHardcodedHandlerDepsToTopology() error {
	if independent == nil || independent.WithHardcodedTopology == nil {
		return fmt.Errorf("service or WithHardcodedTopology is nil")
	}

	for mushroomURL, deps := range independent.handlerDeps {
		serviceConfig, err := independent.topologyHandler.Service(mushroomURL)
		if err != nil {
			return fmt.Errorf("hardcoded handler deps for %q not found in topology: %w", mushroomURL, err)
		}

		for _, dep := range deps {
			serviceConfig.HandlerDeps = setDepService(serviceConfig.HandlerDeps, dep)
		}
		if err := independent.topologyHandler.SetService(serviceConfig, serviceParentURL(mushroomURL)...); err != nil {
			return fmt.Errorf("topologyHandler.SetService(%q): %w", mushroomURL, err)
		}
	}

	return nil
}

func (independent *Independent) addHardcodedCommandDepsToTopology() error {
	if independent == nil || independent.WithHardcodedTopology == nil {
		return fmt.Errorf("service or WithHardcodedTopology is nil")
	}

	for mushroomURL, depsByHandler := range independent.commandDeps {
		serviceConfig, err := independent.topologyHandler.Service(mushroomURL)
		if err != nil {
			return fmt.Errorf("hardcoded command deps for %q not found in topology: %w", mushroomURL, err)
		}

		for handlerCategory, deps := range depsByHandler {
			handlerVariant, err := serviceConfig.HandlerByCategory(handlerCategory)
			if err != nil {
				return fmt.Errorf("hardcoded command deps handler '%s' in service %q: %w", handlerCategory, mushroomURL, err)
			}

			updatedHandler := handlerVariant
			for _, dep := range deps {
				updatedHandler = setHandlerCommandDep(updatedHandler, dep)
			}
			serviceConfig.SetHandler(updatedHandler, true)
		}
		if err := independent.topologyHandler.SetService(serviceConfig, serviceParentURL(mushroomURL)...); err != nil {
			return fmt.Errorf("topologyHandler.SetService(%q): %w", mushroomURL, err)
		}
	}

	return nil
}

func (independent *Extension) addHardcodedServicesToTopology() error {
	if independent == nil || independent.WithHardcodedTopology == nil {
		return fmt.Errorf("service or WithHardcodedTopology is nil")
	}

	for mushroomURL, serviceConfig := range independent.serviceConfigs {
		parent := serviceParentURL(mushroomURL)
		_, err := independent.topologyHandler.Service(mushroomURL)
		if err != nil {
			if err := independent.topologyHandler.AddService(serviceConfig, parent...); err != nil {
				if err := independent.topologyHandler.SetService(serviceConfig, parent...); err != nil {
					return fmt.Errorf("topologyHandler.SetService(%q): %w", mushroomURL, err)
				}
			}
			continue
		}

		if err := independent.topologyHandler.SetService(serviceConfig, parent...); err != nil {
			return fmt.Errorf("topologyHandler.SetService(%q): %w", mushroomURL, err)
		}
	}

	return nil
}

func (independent *Extension) addHardcodedHandlersToTopology() error {
	if independent == nil || independent.WithHardcodedTopology == nil {
		return fmt.Errorf("service or WithHardcodedTopology is nil")
	}

	for mushroomURL, handlers := range independent.handlerConfigs {
		serviceConfig, err := independent.topologyHandler.Service(mushroomURL)
		if err != nil {
			return fmt.Errorf("hardcoded handlers for %q not found in topology: %w", mushroomURL, err)
		}

		for _, handler := range handlers {
			if base, ok := handler.AsIndependentHandler(); ok && base.Category == config.ServiceManagerCategory {
				managerConfig := independent.manager.Config()
				if managerConfig.Endpoint == base.Endpoint {
					continue
				}
				m, err := manager.New(mushroomURL, base.Endpoint)
				if err != nil {
					return fmt.Errorf("manager.New: %w", err)
				}
				independent.manager = m
				if err := independent.manager.SetLogger(independent.logger); err != nil {
					return fmt.Errorf("manager.SetLogger: %w", err)
				}
				continue
			}
			if serviceConfig.Type == config.ProxyType {
				proxyHandler, ok := handler.AsProxyHandler()
				if !ok {
					continue
				}
				handler = normalizeProxyHandlerOutbounds(proxyHandler)
			}
			serviceConfig.SetHandler(handler, true)
		}
		if err := independent.topologyHandler.SetService(serviceConfig, serviceParentURL(mushroomURL)...); err != nil {
			return fmt.Errorf("topologyHandler.SetService(%q): %w", mushroomURL, err)
		}
	}

	return nil
}

func (independent *Extension) addHardcodedHandlerDepsToTopology() error {
	if independent == nil || independent.WithHardcodedTopology == nil {
		return fmt.Errorf("service or WithHardcodedTopology is nil")
	}

	for mushroomURL, deps := range independent.handlerDeps {
		serviceConfig, err := independent.topologyHandler.Service(mushroomURL)
		if err != nil {
			return fmt.Errorf("hardcoded handler deps for %q not found in topology: %w", mushroomURL, err)
		}

		for _, dep := range deps {
			serviceConfig.HandlerDeps = setDepService(serviceConfig.HandlerDeps, dep)
		}
		if err := independent.topologyHandler.SetService(serviceConfig, serviceParentURL(mushroomURL)...); err != nil {
			return fmt.Errorf("topologyHandler.SetService(%q): %w", mushroomURL, err)
		}
	}

	return nil
}

func (independent *Extension) addHardcodedCommandDepsToTopology() error {
	if independent == nil || independent.WithHardcodedTopology == nil {
		return fmt.Errorf("service or WithHardcodedTopology is nil")
	}

	for mushroomURL, depsByHandler := range independent.commandDeps {
		serviceConfig, err := independent.topologyHandler.Service(mushroomURL)
		if err != nil {
			return fmt.Errorf("hardcoded command deps for %q not found in topology: %w", mushroomURL, err)
		}

		for handlerCategory, deps := range depsByHandler {
			handlerVariant, err := serviceConfig.HandlerByCategory(handlerCategory)
			if err != nil {
				return fmt.Errorf("hardcoded command deps handler '%s' in service %q: %w", handlerCategory, mushroomURL, err)
			}

			updatedHandler := handlerVariant
			for _, dep := range deps {
				updatedHandler = setHandlerCommandDep(updatedHandler, dep)
			}
			serviceConfig.SetHandler(updatedHandler, true)
		}
		if err := independent.topologyHandler.SetService(serviceConfig, serviceParentURL(mushroomURL)...); err != nil {
			return fmt.Errorf("topologyHandler.SetService(%q): %w", mushroomURL, err)
		}
	}

	return nil
}

func setHandlerCommandDep(handler config.Handler, dep config.DepService) config.Handler {
	if proxyHandler, ok := handler.AsProxyHandler(); ok {
		proxyHandler.CommandDeps = setDepService(proxyHandler.CommandDeps, dep)
		return proxyHandler
	}
	if independentHandler, ok := handler.AsIndependentHandler(); ok {
		independentHandler.CommandDeps = setDepService(independentHandler.CommandDeps, dep)
		return independentHandler
	}
	return handler
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
