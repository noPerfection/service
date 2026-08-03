package manager

import (
	"fmt"
	"maps"
	"slices"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service/handlers"
	"github.com/noPerfection/service/mushroom"
	"github.com/noPerfection/topology"
	"github.com/noPerfection/topology/config"
)

// RouteCredential is exchanged during manager handshake for routes this service
// exposes as outbounds to a dependency.
type RouteCredential struct {
	// Full route URL either Link or Dereference.
	RouteURL string `json:"route-url"`
	// Handler's public key
	PublicKey string `json:"public-key"`
	// Secret for HMAC authentication
	Secret string `json:"secret"`
}

type routeInboundsHost interface {
	routeInboundsTopology() *topology.Client
	routeInboundsServiceDeref() (string, error)
	routeInboundsMushroomURL() (mushroom.TopologyURL, error)
	routeHandlerCommands(category string) ([]string, error)
}

func (m *Manager) routeInboundsTopology() *topology.Client {
	return m.topology
}

func (m *Manager) routeInboundsServiceDeref() (string, error) {
	return m.serviceURL.AsDereference().String(), nil
}

func (m *Manager) routeInboundsMushroomURL() (mushroom.TopologyURL, error) {
	return m.serviceURL, nil
}

func (m *Manager) routeHandlerCommands(category string) ([]string, error) {
	if category == config.ServiceManagerCategory {
		commands := append([]string(nil), m.Interface.Commands()...)
		return commands, nil
	}
	control, ok := m.handlerControls[category]
	if !ok {
		return nil, fmt.Errorf("handler control of %s category is not found", category)
	}
	return control.Commands()
}

func (m *ProxyManager) routeInboundsTopology() *topology.Client {
	return m.topology
}

func (m *ProxyManager) routeInboundsServiceDeref() (string, error) {
	link, err := m.topology.GetLink(m.serviceName)
	if err != nil {
		return "", fmt.Errorf("topology.GetLink(%q): %w", m.serviceName, err)
	}
	u, err := mushroom.Parse(link)
	if err != nil {
		return "", fmt.Errorf("mushroom.Parse(%q): %w", link, err)
	}
	return u.As(mushroom.SERVICE).AsDereference().String(), nil
}

func (m *ProxyManager) routeInboundsMushroomURL() (mushroom.TopologyURL, error) {
	link, err := m.topology.GetLink(m.serviceName)
	if err != nil {
		return mushroom.TopologyURL{}, fmt.Errorf("topology.GetLink(%q): %w", m.serviceName, err)
	}
	return mushroom.Parse(link)
}

func (m *ProxyManager) routeHandlerCommands(category string) ([]string, error) {
	if category == config.ServiceManagerCategory {
		commands := append([]string(nil), m.Interface.Commands()...)
		return commands, nil
	}

	if err := m.ensureProxyHandlersClient(); err != nil {
		return nil, err
	}

	if err := m.setup.Send(&message.Request{
		Command: handlers.CommandsCommand,
		Parameters: datatype.New().
			Set("category", category),
	}); err != nil {
		return nil, fmt.Errorf("proxyHandlersClient.Send(%q): %w", handlers.CommandsCommand, err)
	}

	reply := <-m.setup.Receive()
	if reply == nil {
		return nil, fmt.Errorf("proxyHandlersClient.Receive(%q): no reply", handlers.CommandsCommand)
	}
	if !reply.IsOK() {
		return nil, fmt.Errorf("proxyHandlersClient.Receive(%q): %s", handlers.CommandsCommand, reply.ErrorMessage())
	}

	rawCommands, err := reply.ReplyParameters().StringsValue("commands")
	if err != nil {
		return nil, fmt.Errorf("reply.ReplyParameters().StringsValue(%q): %w", "commands", err)
	}

	commands := append([]string(nil), rawCommands...)
	return commands, nil
}

// getRouteInbounds returns route to route relationships in this service topology.
// It traverses within dependencies and builds their inbounds defined by this service's topology.
//
//	map(service-link): // Return format:
//		map(route-link): []inbounds
//
// For example: service 1: hello-world; service 2: default-name.
//
//	getRouteInbounds returns:
//	map(hello-world):
//		map(hello-world.main.hello): [default-name.main.hello]
func getRouteInbounds(host routeInboundsHost) (map[string]map[string][]string, error) {
	tp := host.routeInboundsTopology()
	serviceDeref, err := host.routeInboundsServiceDeref()
	if err != nil {
		return nil, err
	}

	serviceConfig, err := tp.Service(serviceDeref)
	if err != nil {
		return nil, fmt.Errorf("topology.Service(%q): %w", serviceDeref, err)
	}

	serviceMushroomURL, err := host.routeInboundsMushroomURL()
	if err != nil {
		return nil, err
	}

	inbounds := make(map[string][]string)

	for _, variant := range serviceConfig.Handlers {
		handler, ok := variant.AsIndependentHandler()
		if !ok {
			return nil, fmt.Errorf("handler %q is not an independent handler", variant)
		}
		handlerInbounds, err := getHandlerInbounds(host, tp, serviceConfig, serviceMushroomURL, handler)
		if err != nil {
			return nil, fmt.Errorf("secure handler edges: %w", err)
		}
		maps.Copy(inbounds, handlerInbounds)
	}

	if _, err := serviceConfig.HandlerByCategory(config.ServiceManagerCategory); err != nil {
		defaultManager, err := DefaultManagerHandlerForService(serviceConfig)
		if err != nil {
			return nil, fmt.Errorf("DefaultManagerHandlerForService: %w", err)
		}
		managerHandler, ok := defaultManager.AsIndependentHandler()
		if !ok {
			return nil, fmt.Errorf("default manager handler is not independent")
		}
		managerInbounds, err := getHandlerInbounds(host, tp, serviceConfig, serviceMushroomURL, managerHandler)
		if err != nil {
			return nil, fmt.Errorf("secure handler edges (manager): %w", err)
		}
		maps.Copy(inbounds, managerInbounds)
	}

	serviceInbounds := make(map[string]map[string][]string)
	for cmdLink := range inbounds {
		cmdMushroomURL, err := mushroom.Parse(cmdLink)
		if err != nil {
			return nil, fmt.Errorf("mushroom.Parse(%q): %w", cmdLink, err)
		}
		serviceLink := cmdMushroomURL.As(mushroom.SERVICE)
		if _, ok := serviceInbounds[serviceLink.String()]; !ok {
			serviceInbounds[serviceLink.String()] = make(map[string][]string)
		}
		serviceInbounds[serviceLink.String()][cmdMushroomURL.String()] = inbounds[cmdLink]
	}

	return serviceInbounds, nil
}

// getHandlerInbounds computes the inbound route map for the given handler and returns it.
// The map key is a route URL (handler hypha + command prop), the value is the list of
// route URLs that are allowed to call that route.
func getHandlerInbounds(host routeInboundsHost, tp *topology.Client, serviceConfig config.Service, serviceMushroomURL mushroom.TopologyURL, handler config.IndependentHandler) (map[string][]string, error) {
	cmds, err := host.routeHandlerCommands(handler.Category)
	if err != nil {
		return nil, fmt.Errorf("commands(%q): %w", handler.Category, err)
	}
	handlerURL := serviceMushroomURL.New(handler.Category)

	routes := make(map[string][]string)

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

	var handlerDep *config.DepService
	for i := range serviceConfig.HandlerDeps {
		if serviceConfig.HandlerDeps[i].Name == handler.Category {
			d := serviceConfig.HandlerDeps[i]
			handlerDep = &d
			break
		}
	}

	for _, cmd := range cmds {
		cmdRouteURL := handlerURL.NewRouteURL(cmd)

		if proxies, hasProxies := cmdProxies[cmd]; hasProxies {
			if _, exists := routes[cmdRouteURL.String()]; !exists {
				routes[cmdRouteURL.String()] = []string{}
			}

			proxyLink, err := tp.GetLink(proxies[len(proxies)-1])
			if err != nil {
				return nil, fmt.Errorf("command %q last proxy link: %w", cmd, err)
			}
			proxyMushroomURL, err := mushroom.Parse(proxyLink)
			if err != nil {
				return nil, fmt.Errorf("mushroom.Parse(%q): %w", proxyLink, err)
			}
			lastProxyURL := proxyMushroomURL.NewRouteURL(cmd)

			routes[cmdRouteURL.String()] = appendUnique(routes[cmdRouteURL.String()], lastProxyURL.String())
			if err := buildCommandProxyChain(tp, routes, proxies, cmd, handlerDep); err != nil {
				return nil, fmt.Errorf("command %q proxy chain: %w", cmd, err)
			}
		}

		if exts, hasExts := cmdExtensions[cmd]; hasExts {
			for _, ext := range exts {
				extLink, err := tp.GetLink(ext)
				if err != nil {
					return nil, fmt.Errorf("command %q extension link: %w", cmd, err)
				}
				extMushroomURL, err := mushroom.Parse(extLink)
				if err != nil {
					return nil, fmt.Errorf("mushroom.Parse(%q): %w", extLink, err)
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
					otherMushroomURL, err := mushroom.Parse(otherLink)
					if err != nil {
						return nil, fmt.Errorf("mushroom.Parse(%q): %w", otherLink, err)
					}
					otherURL := otherMushroomURL.NewRouteURL(cmd)
					routes[extRouteURL.String()] = appendUnique(routes[extRouteURL.String()], otherURL.String())
				}
				routes[extRouteURL.String()] = appendUnique(routes[extRouteURL.String()], cmdRouteURL.String())
			}
		}

		if _, hasProxies := cmdProxies[cmd]; !hasProxies {
			if handlerDep != nil && len(handlerDep.Proxies) > 0 {
				if _, exists := routes[cmdRouteURL.String()]; !exists {
					routes[cmdRouteURL.String()] = []string{}
				}

				lastHdLink, err := tp.GetLink(handlerDep.Proxies[len(handlerDep.Proxies)-1])
				if err != nil {
					return nil, fmt.Errorf("command %q handler dep last proxy link: %w", cmd, err)
				}
				lastHdMushroomURL, err := mushroom.Parse(lastHdLink)
				if err != nil {
					return nil, fmt.Errorf("mushroom.Parse(%q): %w", lastHdLink, err)
				}
				lastHdURL := lastHdMushroomURL.NewRouteURL(cmd)

				routes[cmdRouteURL.String()] = appendUnique(routes[cmdRouteURL.String()], lastHdURL.String())
			}
		}
	}

	if handlerDep != nil {
		hdProxies := handlerDep.Proxies
		for i := 1; i < len(hdProxies); i++ {
			currLink, err := tp.GetLink(hdProxies[i])
			if err != nil {
				return nil, fmt.Errorf("handler dep proxy[%d] link: %w", i, err)
			}
			currMushroomURL, err := mushroom.Parse(currLink)
			if err != nil {
				return nil, fmt.Errorf("mushroom.Parse(%q): %w", currLink, err)
			}
			currURL := currMushroomURL.NewRouteURL(message.Any)

			prevLink, err := tp.GetLink(hdProxies[i-1])
			if err != nil {
				return nil, fmt.Errorf("handler dep proxy[%d] link: %w", i-1, err)
			}
			prevMushroomURL, err := mushroom.Parse(prevLink)
			if err != nil {
				return nil, fmt.Errorf("mushroom.Parse(%q): %w", prevLink, err)
			}
			prevURL := prevMushroomURL.NewRouteURL(message.Any)

			if _, exists := routes[currURL.String()]; !exists {
				routes[currURL.String()] = []string{}
			}
			routes[currURL.String()] = appendUnique(routes[currURL.String()], prevURL.String())
		}

		handlerAnyURL := handlerURL.NewRouteURL(message.Any)
		for _, ext := range handlerDep.Extensions {
			extLink, err := tp.GetLink(ext)
			if err != nil {
				return nil, fmt.Errorf("handler dep extension link: %w", err)
			}
			extMushroomURL, err := mushroom.Parse(extLink)
			if err != nil {
				return nil, fmt.Errorf("mushroom.Parse(%q): %w", extLink, err)
			}
			extURL := extMushroomURL.NewRouteURL(message.Any)

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
				otherMushroomURL, err := mushroom.Parse(otherLink)
				if err != nil {
					return nil, fmt.Errorf("mushroom.Parse(%q): %w", otherLink, err)
				}
				otherURL := otherMushroomURL.NewRouteURL(message.Any)
				routes[extURL.String()] = appendUnique(routes[extURL.String()], otherURL.String())
			}
		}
	}

	return routes, nil
}

func buildCommandProxyChain(tp *topology.Client, routes map[string][]string, proxies []string, cmd string, handlerDep *config.DepService) error {
	for i := len(proxies) - 1; i >= 1; i-- {
		currLink, err := tp.GetLink(proxies[i])
		if err != nil {
			return fmt.Errorf("proxy[%d] link: %w", i, err)
		}
		currMushroomURL, err := mushroom.Parse(currLink)
		if err != nil {
			return fmt.Errorf("proxy[%d] mushroom url: %w", i, err)
		}
		currMushroomURL = currMushroomURL.HandlerLink()
		currURL := currMushroomURL.NewRouteURL(cmd)

		prevLink, err := tp.GetLink(proxies[i-1])
		if err != nil {
			return fmt.Errorf("proxy[%d] link: %w", i-1, err)
		}
		prevMushroomURL, err := mushroom.Parse(prevLink)
		if err != nil {
			return fmt.Errorf("mushroom.Parse(%q): %w", prevLink, err)
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
		firstMushroomURL, err := mushroom.Parse(firstLink)
		if err != nil {
			return fmt.Errorf("mushroom.Parse(%q): %w", firstLink, err)
		}
		firstURL := firstMushroomURL.NewRouteURL(cmd)

		lastHdProxy := handlerDep.Proxies[len(handlerDep.Proxies)-1]
		lastHdLink, err := tp.GetLink(lastHdProxy)
		if err != nil {
			return fmt.Errorf("handler dep last proxy link: %w", err)
		}
		lastHdMushroomURL, err := mushroom.Parse(lastHdLink)
		if err != nil {
			return fmt.Errorf("mushroom.Parse(%q): %w", lastHdLink, err)
		}
		lastHdURL := lastHdMushroomURL.NewRouteURL(cmd)

		if _, exists := routes[firstURL.String()]; !exists {
			routes[firstURL.String()] = []string{}
		}
		routes[firstURL.String()] = appendUnique(routes[firstURL.String()], lastHdURL.String())
	}
	return nil
}

func appendUnique(values []string, value string) []string {
	if slices.Contains(values, value) {
		return values
	}
	return append(values, value)
}

func asTopologyURL(serviceURL string, tp *topology.Client) (mushroom.TopologyURL, error) {
	link := serviceURL
	if tp != nil {
		if resolved, err := tp.GetLink(serviceURL); err == nil {
			link = resolved
		}
	}
	return mushroom.Parse(link)
}
