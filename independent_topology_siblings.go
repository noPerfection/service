package service

import (
	"fmt"

	"github.com/ahmetson/mushroom"
	"github.com/noPerfection/log"
	"github.com/noPerfection/protocol/handler/base"
	"github.com/noPerfection/topology"
	"github.com/noPerfection/topology/config"
)

// secureEdges prints who may call each handler command on this independent
// service by walking command-deps, handler-deps, and registered routes.
func (independent *Independent) secureEdges() error {
	serviceConfig, err := independent.topology().Service(independent.mushroomURL)
	if err != nil {
		return fmt.Errorf("topology.Service(%q): %w", independent.mushroomURL, err)
	}

	logger := independent.logger
	if independent.logger == nil {
		var err error
		logger, err = log.New(serviceConfig.Name, true)
		if err != nil {
			return fmt.Errorf("log.New(%s): %w", serviceConfig.Name, err)
		}
	}

	logger.Info("Collect for handlers of", "MushroomURL", independent.mushroomURL, "handlers amount", len(serviceConfig.Handlers))

	for _, variant := range serviceConfig.Handlers {
		fmt.Printf("\n")
		handler, ok := variant.AsIndependentHandler()
		if !ok {
			return fmt.Errorf("handler %q is not an independent handler", variant)
		}
		cmds, err := independent.commands(handler.Category)
		if err != nil {
			return fmt.Errorf("commands(%q): %w", handler.Category, err)
		}
		logger.Info("Collecting inbound edges for handler:\n", "\tcategory", handler.Category, "\n\troutes amount", len(cmds))

		inbounds, err := independent.secureHandlerEdges(serviceConfig, handler)
		if err != nil {
			return fmt.Errorf("secure handler edges: %w", err)
		}
		logger.Info("Inbound edges collected:\n", "\tinbounds amount", len(inbounds))
		for cmd, inbounds := range inbounds {
			logger.Info("Inbound for\n", "\tcommand", cmd, "\n\tinbounds", inbounds)
		}
	}

	return nil
}

// commands returns the registered route commands for the given handler category.
// For ServiceManagerCategory it delegates to the embedded manager interface;
// for all other categories it uses the application Handlers registry.
func (independent *Independent) commands(category string) ([]string, error) {
	if category == topology.ServiceManagerCategory {
		if independent.manager == nil {
			return nil, fmt.Errorf("manager is nil")
		}
		return independent.manager.Commands(), nil
	}
	return independent.Handlers.RouteCommands(category)
}

// secureHandlerEdges computes the inbound route map for the given handler and returns it.
// The map key is a route URL (handler hypha + command prop), the value is the list of
// route URLs that are allowed to call that route.
func (independent *Independent) secureHandlerEdges(serviceConfig config.Service, handler config.IndependentHandler) (map[string][]string, error) {
	cmds, err := independent.commands(handler.Category)
	if err != nil {
		return nil, fmt.Errorf("commands(%q): %w", handler.Category, err)
	}
	handlerURL, err := independent.GetHandlerLink(handler.Category)
	if err != nil {
		return nil, fmt.Errorf("GetHandlerLink(%q): %w", handler.Category, err)
	}

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

	for _, cmd := range cmds {
		cmdRouteURL := handlerHyphaWithCmd(handlerURL, cmd)

		// Command has proxy deps: last proxy calls the service; chain the rest.
		if proxies, hasProxies := cmdProxies[cmd]; hasProxies {
			if _, exists := routes[cmdRouteURL]; !exists {
				routes[cmdRouteURL] = []string{}
			}
			lastProxyURL, err := independent.depCmdRouteURL(proxies[len(proxies)-1], cmd)
			if err != nil {
				return nil, fmt.Errorf("command %q last proxy route url: %w", cmd, err)
			}
			routes[cmdRouteURL] = appendUnique(routes[cmdRouteURL], lastProxyURL)
			if err := independent.buildCommandProxyChain(routes, proxies, cmd, handlerDep); err != nil {
				return nil, fmt.Errorf("command %q proxy chain: %w", cmd, err)
			}
		}

		// Command has extension deps: each extension's inbounds are its siblings + the service.
		if exts, hasExts := cmdExtensions[cmd]; hasExts {
			for _, ext := range exts {
				extRouteURL, err := independent.depCmdRouteURL(ext, cmd)
				if err != nil {
					return nil, fmt.Errorf("command %q extension route url: %w", cmd, err)
				}
				if _, exists := routes[extRouteURL]; !exists {
					routes[extRouteURL] = []string{}
				}
				for _, other := range exts {
					if other == ext {
						continue
					}
					otherURL, err := independent.depCmdRouteURL(other, cmd)
					if err != nil {
						return nil, fmt.Errorf("command %q sibling extension route url: %w", cmd, err)
					}
					routes[extRouteURL] = appendUnique(routes[extRouteURL], otherURL)
				}
				routes[extRouteURL] = appendUnique(routes[extRouteURL], cmdRouteURL)
			}
		}

		// No command dep proxies: fall back to the handler dep's last proxy as inbound.
		if _, hasProxies := cmdProxies[cmd]; !hasProxies {
			if handlerDep != nil && len(handlerDep.Proxies) > 0 {
				if _, exists := routes[cmdRouteURL]; !exists {
					routes[cmdRouteURL] = []string{}
				}
				lastHdProxy := handlerDep.Proxies[len(handlerDep.Proxies)-1]
				lastHdURL, err := independent.depCmdRouteURL(lastHdProxy, cmd)
				if err != nil {
					return nil, fmt.Errorf("command %q handler dep last proxy route url: %w", cmd, err)
				}
				routes[cmdRouteURL] = appendUnique(routes[cmdRouteURL], lastHdURL)
			}
		}
	}

	// Handler dep proxy chain: each proxy[i]'s base.Any route has proxy[i-1] as inbound.
	if handlerDep != nil {
		hdProxies := handlerDep.Proxies
		for i := 1; i < len(hdProxies); i++ {
			currURL, err := independent.depCmdRouteURL(hdProxies[i], base.Any)
			if err != nil {
				return nil, fmt.Errorf("handler dep proxy[%d] route url: %w", i, err)
			}
			prevURL, err := independent.depCmdRouteURL(hdProxies[i-1], base.Any)
			if err != nil {
				return nil, fmt.Errorf("handler dep proxy[%d] route url: %w", i-1, err)
			}
			if _, exists := routes[currURL]; !exists {
				routes[currURL] = []string{}
			}
			routes[currURL] = appendUnique(routes[currURL], prevURL)
		}

		// Handler dep extensions: each ext's inbounds are this handler's base.Any URL + sibling ext base.Any URLs.
		handlerAnyURL := handlerHyphaWithCmd(handlerURL, base.Any)
		for _, ext := range handlerDep.Extensions {
			extURL, err := independent.depCmdRouteURL(ext, base.Any)
			if err != nil {
				return nil, fmt.Errorf("handler dep extension route url: %w", err)
			}
			if _, exists := routes[extURL]; !exists {
				routes[extURL] = []string{}
			}
			routes[extURL] = appendUnique(routes[extURL], handlerAnyURL)
			for _, other := range handlerDep.Extensions {
				if other == ext {
					continue
				}
				otherURL, err := independent.depCmdRouteURL(other, base.Any)
				if err != nil {
					return nil, fmt.Errorf("handler dep sibling extension route url: %w", err)
				}
				routes[extURL] = appendUnique(routes[extURL], otherURL)
			}
		}
	}

	return routes, nil
}

// buildCommandProxyChain wires inbound edges within a command's proxy chain.
// Each proxy[i] is set as the inbound of proxy[i+1]. If a handler dep is provided,
// the last handler dep proxy becomes the inbound of proxy[0].
func (independent *Independent) buildCommandProxyChain(routes map[string][]string, proxies []string, cmd string, handlerDep *config.DepService) error {
	for i := len(proxies) - 1; i >= 1; i-- {
		currURL, err := independent.depCmdRouteURL(proxies[i], cmd)
		if err != nil {
			return fmt.Errorf("proxy[%d] route url: %w", i, err)
		}
		prevURL, err := independent.depCmdRouteURL(proxies[i-1], cmd)
		if err != nil {
			return fmt.Errorf("proxy[%d] route url: %w", i-1, err)
		}
		if _, exists := routes[currURL]; !exists {
			routes[currURL] = []string{}
		}
		routes[currURL] = appendUnique(routes[currURL], prevURL)
	}
	if len(proxies) > 0 && handlerDep != nil && len(handlerDep.Proxies) > 0 {
		firstURL, err := independent.depCmdRouteURL(proxies[0], cmd)
		if err != nil {
			return fmt.Errorf("first proxy route url: %w", err)
		}
		lastHdProxy := handlerDep.Proxies[len(handlerDep.Proxies)-1]
		lastHdURL, err := independent.depCmdRouteURL(lastHdProxy, cmd)
		if err != nil {
			return fmt.Errorf("handler dep last proxy route url: %w", err)
		}
		if _, exists := routes[firstURL]; !exists {
			routes[firstURL] = []string{}
		}
		routes[firstURL] = appendUnique(routes[firstURL], lastHdURL)
	}
	return nil
}

// depCmdRouteURL resolves the handler-link route URL for a dep service and command.
func (independent *Independent) depCmdRouteURL(mushroomURL, cmd string) (string, error) {
	tp := independent.topology()
	link, err := tp.GetLink(dereferenceMushroomURL(mushroomURL))
	if err != nil {
		return "", fmt.Errorf("GetLink(%q): %w", mushroomURL, err)
	}
	category := handlerCategoryFromMushroomURL(mushroomURL)
	var soil mushroom.Soil
	hypha, err := soil.Hypha(link)
	if err != nil {
		return "", fmt.Errorf("soil.Hypha(%q): %w", link, err)
	}
	lh := hypha.AsLink()
	if lh.AdditionalProps == nil {
		lh.AdditionalProps = map[string]string{}
	}
	lh.AdditionalProps["category"] = category
	lh.AdditionalProps["command"] = cmd
	return lh.String(), nil
}

// handlerHyphaWithCmd parses handlerURL and returns it with AdditionalProps["command"] set to cmd.
func handlerHyphaWithCmd(handlerURL, cmd string) string {
	var soil mushroom.Soil
	h, err := soil.Hypha(handlerURL)
	if err != nil {
		return handlerURL
	}
	if h.AdditionalProps == nil {
		h.AdditionalProps = map[string]string{}
	}
	h.AdditionalProps["command"] = cmd
	return h.String()
}
