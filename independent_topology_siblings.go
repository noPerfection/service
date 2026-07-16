package service

import (
	"fmt"
	"maps"

	"github.com/noPerfection/log"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service/mushroom"
	"github.com/noPerfection/topology/config"
)

// secureEdges prints who may call each handler command on this independent
// service by walking command-deps, handler-deps, and registered routes.
//
// TODO:
// Then, we create a node interface in topology a new method: SecureEdges(serviceURL)
// Then, there is an interface called SetInbounds(serviceURL, inbounds) ?? how to retreive it after restart?
// From whom it gets the message?
//
// Then managers implement it.
// Then handlers has the inbounds and can be called by manager.
// The manager creates a whitelist and secure keys.
//
// Perhaps we need to set the secure edges for the topology as well?
func (independent *Independent) secureEdges() error {
	serviceConfig, err := independent.topology().Service(independent.dereference())
	if err != nil {
		return fmt.Errorf("topology.Service(%q): %w", independent.dereference(), err)
	}
	logger := independent.logger
	if independent.logger == nil {
		var err error
		logger, err = log.New(serviceConfig.Name, true)
		if err != nil {
			return fmt.Errorf("log.New(%s): %w", serviceConfig.Name, err)
		}
	}

	inbounds := make(map[string][]string)

	for _, variant := range serviceConfig.Handlers {
		fmt.Printf("\n")
		handler, ok := variant.AsIndependentHandler()
		if !ok {
			return fmt.Errorf("handler %q is not an independent handler", variant)
		}
		handlerInbounds, err := independent.secureHandlerEdges(serviceConfig, handler)
		if err != nil {
			return fmt.Errorf("secure handler edges: %w", err)
		}
		maps.Copy(inbounds, handlerInbounds)
	}
	// Here we need to filter the inbounds per service
	serviceInbounds := make(map[string]map[string][]string)
	for cmdLink := range inbounds {
		cmdMushroomURL, err := mushroom.New(cmdLink)
		if err != nil {
			return fmt.Errorf("mushroom.New(%q): %w", cmdLink, err)
		}
		serviceLink := cmdMushroomURL.As(mushroom.SERVICE)
		_, ok := serviceInbounds[serviceLink.String()]
		if !ok {
			serviceInbounds[serviceLink.String()] = make(map[string][]string)
		}
		serviceInbounds[serviceLink.String()][cmdMushroomURL.String()] = inbounds[cmdLink]
	}

	// Now we print all:
	logger.Info("Service inbounds", "services amount", len(serviceInbounds))
	for serviceLink, cmdInbounds := range serviceInbounds {
		logger.Info("Service inbounds \n", "\tservice", serviceLink, "cmd amount", len(cmdInbounds))
		for cmd, inbounds := range cmdInbounds {
			logger.Info("Inbounds for\n", "\tcommand", cmd, "\n\tinbounds", inbounds)
		}
		fmt.Printf("\n")
	}

	return nil
}

// commands returns the registered route commands for the given handler category.
// For ServiceManagerCategory it delegates to the embedded manager interface;
// for all other categories it uses the application Handlers registry.
func (independent *Independent) commands(category string) ([]string, error) {
	if category == config.ServiceManagerCategory {
		if independent.manager == nil {
			return nil, fmt.Errorf("manager is nil")
		}
		return independent.manager.Commands(), nil
	}
	return independent.Setup.RouteCommands(category)
}

// secureHandlerEdges computes the inbound route map for the given handler and returns it.
// The map key is a route URL (handler hypha + command prop), the value is the list of
// route URLs that are allowed to call that route.
func (independent *Independent) secureHandlerEdges(serviceConfig config.Service, handler config.IndependentHandler) (map[string][]string, error) {
	cmds, err := independent.commands(handler.Category)
	if err != nil {
		return nil, fmt.Errorf("commands(%q): %w", handler.Category, err)
	}
	handlerURL := independent.mushroomURL.New(handler.Category)

	routes := make(map[string][]string)

	// Index command deps by command name.
	cmdProxies := make(map[string][]string)
	cmdExtensions := make(map[string][]string)
	for _, dep := range handler.CommandDeps {
		if len(dep.Proxies) > 0 {
			cmdProxies[dep.Name] = dep.Proxies
		}
		if len(dep.Extensions) > 0 {
			cmdExtensions[dep.Name] = dep.Extensions
		}
	}

	// Find the handler dep for this handler category (nil if not set).
	var handlerDep *config.DepService
	for i := range serviceConfig.HandlerDeps {
		if serviceConfig.HandlerDeps[i].Name == handler.Category {
			d := serviceConfig.HandlerDeps[i]
			handlerDep = &d
			break
		}
	}

	tp := independent.topology()

	for _, cmd := range cmds {

		cmdRouteURL := handlerURL.NewRouteURL(cmd)

		// Command has proxy deps: last proxy calls the service; chain the rest.
		if proxies, hasProxies := cmdProxies[cmd]; hasProxies {
			if _, exists := routes[cmdRouteURL.String()]; !exists {
				routes[cmdRouteURL.String()] = []string{}
			}

			proxyLink, err := tp.GetLink(proxies[len(proxies)-1])
			if err != nil {
				return nil, fmt.Errorf("command %q last proxy link: %w", cmd, err)
			}
			proxyMushroomURL, err := mushroom.New(proxyLink)
			if err != nil {
				return nil, fmt.Errorf("mushroom.New(%q): %w", proxyLink, err)
			}
			lastProxyURL := proxyMushroomURL.NewRouteURL(cmd)

			routes[cmdRouteURL.String()] = appendUnique(routes[cmdRouteURL.String()], lastProxyURL.String())
			if err := independent.buildCommandProxyChain(routes, proxies, cmd, handlerDep); err != nil {
				return nil, fmt.Errorf("command %q proxy chain: %w", cmd, err)
			}
		}

		// Command has extension deps: each extension's inbounds are its siblings + the service.
		if exts, hasExts := cmdExtensions[cmd]; hasExts {
			for _, ext := range exts {
				extLink, err := tp.GetLink(ext)
				if err != nil {
					return nil, fmt.Errorf("command %q extension link: %w", cmd, err)
				}
				extMushroomURL, err := mushroom.New(extLink)
				if err != nil {
					return nil, fmt.Errorf("mushroom.New(%q): %w", extLink, err)
				}
				extRouteURL := extMushroomURL.NewRouteURL(cmd)
				if _, exists := routes[extRouteURL.String()]; !exists {
					routes[extRouteURL.String()] = []string{}
				}
				for _, other := range exts {
					if other == ext {
						continue
					}
					otherLink, err := tp.GetLink(other)
					if err != nil {
						return nil, fmt.Errorf("command %q sibling extension link: %w", cmd, err)
					}
					otherMushroomURL, err := mushroom.New(otherLink)
					if err != nil {
						return nil, fmt.Errorf("mushroom.New(%q): %w", otherLink, err)
					}
					otherURL := otherMushroomURL.NewRouteURL(cmd)
					if err != nil {
						return nil, fmt.Errorf("command %q sibling extension route url: %w", cmd, err)
					}
					routes[extRouteURL.String()] = appendUnique(routes[extRouteURL.String()], otherURL.String())
				}
				routes[extRouteURL.String()] = appendUnique(routes[extRouteURL.String()], cmdRouteURL.String())
			}
		}

		// No command dep proxies: fall back to the handler dep's last proxy as inbound.
		if _, hasProxies := cmdProxies[cmd]; !hasProxies {
			if handlerDep != nil && len(handlerDep.Proxies) > 0 {
				if _, exists := routes[cmdRouteURL.String()]; !exists {
					routes[cmdRouteURL.String()] = []string{}
				}

				lastHdLink, err := tp.GetLink(handlerDep.Proxies[len(handlerDep.Proxies)-1])
				if err != nil {
					return nil, fmt.Errorf("command %q handler dep last proxy link: %w", cmd, err)
				}
				lastHdMushroomURL, err := mushroom.New(lastHdLink)
				if err != nil {
					return nil, fmt.Errorf("mushroom.New(%q): %w", lastHdLink, err)
				}
				lastHdURL := lastHdMushroomURL.NewRouteURL(cmd)

				routes[cmdRouteURL.String()] = appendUnique(routes[cmdRouteURL.String()], lastHdURL.String())
			}
		}
	}

	// Handler dep proxy chain: each proxy[i]'s message.Any route has proxy[i-1] as inbound.
	if handlerDep != nil {
		hdProxies := handlerDep.Proxies
		for i := 1; i < len(hdProxies); i++ {
			currLink, err := tp.GetLink(hdProxies[i])
			if err != nil {
				return nil, fmt.Errorf("handler dep proxy[%d] link: %w", i, err)
			}
			currMushroomURL, err := mushroom.New(currLink)
			if err != nil {
				return nil, fmt.Errorf("mushroom.New(%q): %w", currLink, err)
			}
			currURL := currMushroomURL.NewRouteURL(message.Any)
			if err != nil {
				return nil, fmt.Errorf("handler dep proxy[%d] route url: %w", i, err)
			}
			prevLink, err := tp.GetLink(hdProxies[i-1])
			if err != nil {
				return nil, fmt.Errorf("handler dep proxy[%d] link: %w", i-1, err)
			}
			prevMushroomURL, err := mushroom.New(prevLink)
			if err != nil {
				return nil, fmt.Errorf("mushroom.New(%q): %w", prevLink, err)
			}
			prevURL := prevMushroomURL.NewRouteURL(message.Any)
			if err != nil {
				return nil, fmt.Errorf("handler dep proxy[%d] route url: %w", i-1, err)
			}
			if _, exists := routes[currURL.String()]; !exists {
				routes[currURL.String()] = []string{}
			}
			routes[currURL.String()] = appendUnique(routes[currURL.String()], prevURL.String())
		}

		// Handler dep extensions: each ext's inbounds are this handler's message.Any URL + sibling ext message.Any URLs.
		handlerAnyURL := handlerURL.NewRouteURL(message.Any)
		for _, ext := range handlerDep.Extensions {
			extLink, err := tp.GetLink(ext)
			if err != nil {
				return nil, fmt.Errorf("handler dep extension link: %w", err)
			}
			extMushroomURL, err := mushroom.New(extLink)
			if err != nil {
				return nil, fmt.Errorf("mushroom.New(%q): %w", extLink, err)
			}
			extURL := extMushroomURL.NewRouteURL(message.Any)
			if err != nil {
				return nil, fmt.Errorf("handler dep extension route url: %w", err)
			}
			if _, exists := routes[extURL.String()]; !exists {
				routes[extURL.String()] = []string{}
			}
			routes[extURL.String()] = appendUnique(routes[extURL.String()], handlerAnyURL.String())
			for _, other := range handlerDep.Extensions {
				if other == ext {
					continue
				}
				otherLink, err := tp.GetLink(other)
				if err != nil {
					return nil, fmt.Errorf("handler dep sibling extension link: %w", err)
				}
				otherMushroomURL, err := mushroom.New(otherLink)
				if err != nil {
					return nil, fmt.Errorf("mushroom.New(%q): %w", otherLink, err)
				}
				otherURL := otherMushroomURL.NewRouteURL(message.Any)
				if err != nil {
					return nil, fmt.Errorf("handler dep sibling extension route url: %w", err)
				}
				routes[extURL.String()] = appendUnique(routes[extURL.String()], otherURL.String())
			}
		}
	}

	return routes, nil
}

// buildCommandProxyChain wires inbound edges within a command's proxy chain.
// Each proxy[i] is set as the inbound of proxy[i+1]. If a handler dep is provided,
// the last handler dep proxy becomes the inbound of proxy[0].
func (independent *Independent) buildCommandProxyChain(routes map[string][]string, proxies []string, cmd string, handlerDep *config.DepService) error {
	tp := independent.topology()
	for i := len(proxies) - 1; i >= 1; i-- {
		currLink, err := tp.GetLink(proxies[i])
		if err != nil {
			return fmt.Errorf("proxy[%d] link: %w", i, err)
		}
		currMushroomURL, err := mushroom.New(currLink)
		if err != nil {
			return fmt.Errorf("proxy[%d] mushroom url: %w", i, err)
		}
		currMushroomURL = currMushroomURL.HandlerLink()
		currURL := currMushroomURL.NewRouteURL(cmd)
		prevLink, err := tp.GetLink(proxies[i-1])
		if err != nil {
			return fmt.Errorf("proxy[%d] link: %w", i-1, err)
		}
		prevMushroomURL, err := mushroom.New(prevLink)
		if err != nil {
			return fmt.Errorf("mushroom.New(%q): %w", prevLink, err)
		}
		prevURL := prevMushroomURL.NewRouteURL(cmd)
		if _, exists := routes[currURL.String()]; !exists {
			routes[currURL.String()] = []string{}
		}
		routes[currURL.String()] = appendUnique(routes[currURL.String()], prevURL.String())
	}
	if len(proxies) > 0 && handlerDep != nil && len(handlerDep.Proxies) > 0 {
		firstLink, err := tp.GetLink(proxies[0])
		if err != nil {
			return fmt.Errorf("first proxy link: %w", err)
		}
		firstMushroomURL, err := mushroom.New(firstLink)
		if err != nil {
			return fmt.Errorf("mushroom.New(%q): %w", firstLink, err)
		}
		firstURL := firstMushroomURL.NewRouteURL(cmd)
		if err != nil {
			return fmt.Errorf("first proxy route url: %w", err)
		}
		lastHdProxy := handlerDep.Proxies[len(handlerDep.Proxies)-1]
		lastHdLink, err := tp.GetLink(lastHdProxy)
		if err != nil {
			return fmt.Errorf("handler dep last proxy link: %w", err)
		}
		lastHdMushroomURL, err := mushroom.New(lastHdLink)
		if err != nil {
			return fmt.Errorf("mushroom.New(%q): %w", lastHdLink, err)
		}
		lastHdURL := lastHdMushroomURL.NewRouteURL(cmd)
		if err != nil {
			return fmt.Errorf("handler dep last proxy route url: %w", err)
		}
		if _, exists := routes[firstURL.String()]; !exists {
			routes[firstURL.String()] = []string{}
		}
		routes[firstURL.String()] = appendUnique(routes[firstURL.String()], lastHdURL.String())
	}
	return nil
}
