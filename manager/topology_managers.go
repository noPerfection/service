package manager

import (
	"fmt"
	"strings"
	"time"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/protocol/client"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service/handlers"
	"github.com/noPerfection/service/mushroom"
	"github.com/noPerfection/topology"
	"github.com/noPerfection/topology/config"
)

// inprocTopologyEndpoint is the endpoint of the inproc topology extension service.
func startInprocService(inprocTopologyEndpoint message.Endpoint, handlerType config.HandlerType, serviceName string) (string, error) {
	socket, err := client.New(inprocTopologyEndpoint.Id, inprocTopologyEndpoint.Port, client.HandlerType(handlerType))
	if err != nil {
		return "", fmt.Errorf("client.New: %w", err)
	}
	defer socket.Close()

	socket.Timeout(time.Second)
	socket.Attempt(3)

	reply, err := socket.Request(&message.Request{
		Command:    StartService,
		Parameters: datatype.New().Set("service", serviceName),
	})
	if err != nil {
		return "", fmt.Errorf("socket.Request('%s'): %w", StartService, err)
	}
	if !reply.IsOK() {
		return "", fmt.Errorf("reply.Message: %s", reply.ErrorMessage())
	}

	id, err := reply.ReplyParameters().StringValue("id")
	if err != nil {
		return "", fmt.Errorf("reply.Parameters.GetString('id'): %w", err)
	}
	return id, nil
}

func inprocTopologyExtensionEndpoint(topologyClient *topology.Client) (message.Endpoint, config.HandlerType, error) {
	if topologyClient == nil {
		return message.Endpoint{}, "", fmt.Errorf("topology is nil")
	}
	record, err := topologyClient.Service(InprocTopologyServiceName)
	if err != nil {
		return message.Endpoint{}, "", fmt.Errorf("topology.Service(%q): %w", InprocTopologyServiceName, err)
	}
	extensionHandler, err := record.HandlerByCategory(handlers.DefaultHandlerCategory)
	if err != nil {
		return message.Endpoint{}, "", fmt.Errorf("inproc topology extension handler: %w", err)
	}
	handler, ok := extensionHandler.AsIndependentHandler()
	if !ok {
		return message.Endpoint{}, "", fmt.Errorf("inproc topology extension handler is not independent")
	}
	return handler.Endpoint, handler.Type, nil
}

func getPublicKeyFromConfig(inboundURL string, tp *topology.Client) (string, error) {
	u, err := mushroom.Parse(inboundURL)
	if err != nil {
		return "", fmt.Errorf("mushroom.Parse(%q): %w", inboundURL, err)
	}
	serviceURL := u.As(mushroom.SERVICE).AsDereference().String()
	if tp != nil {
		if err := tp.Reload(); err != nil {
			return "", fmt.Errorf("topology.Reload: %w", err)
		}
		if link, err := tp.GetLink(serviceURL); err == nil {
			if resolved, err := mushroom.Parse(link); err == nil {
				serviceURL = resolved.As(mushroom.SERVICE).AsDereference().String()
			}
		}
	}

	service, err := tp.Service(serviceURL)
	if err != nil {
		return "", fmt.Errorf("topology.Service(%q): %w", serviceURL, err)
	}
	if service.Parameters != nil {
		if pubKey, ok := service.Parameters[ManagerPublicKeyParam].(string); ok && pubKey != "" {
			return pubKey, nil
		}
	}

	serviceLink, err := tp.GetLink(serviceURL)
	if err != nil {
		return "", fmt.Errorf("service %q has no %q parameter", service.Name, ManagerPublicKeyParam)
	}
	resolvedService, err := mushroom.Parse(serviceLink)
	if err != nil {
		return "", fmt.Errorf("mushroom.Parse(%q): %w", serviceLink, err)
	}
	pubKeyLink, err := tp.GetLink(resolvedService.As(mushroom.SERVICE).ResourcePublicKey().String())
	if err != nil {
		return "", fmt.Errorf("service %q has no %q parameter", service.Name, ManagerPublicKeyParam)
	}
	pubKey := pubKeyLink
	if strings.HasPrefix(pubKeyLink, "*") || strings.HasPrefix(pubKeyLink, "pkg:") {
		if resolved, err := tp.GetLink(pubKeyLink); err == nil && resolved != "" {
			pubKey = resolved
		}
	}
	if pubKey == "" {
		return "", fmt.Errorf("service %q has no %q parameter", service.Name, ManagerPublicKeyParam)
	}
	return pubKey, nil
}

// filterTopologyOutbounds filters out topologyOutbounds where serviceURL has outboundServiceURL as its outbound.
// If excludeOutboundService is true, then it does reverse, filters all except outboundServiceURL.
func filterTopologyOutbounds(topologyOutbounds map[string]map[string]string, serviceURL, outboundServiceURL mushroom.TopologyURL, excludeOutboundService bool) (map[string]mushroom.TopologyURL, error) {
	allOutbounds, hasOutbounds := topologyOutbounds[serviceURL.AsDereference().String()]

	outbounds := make(map[string]mushroom.TopologyURL)
	if len(allOutbounds) == 0 || !hasOutbounds {
		return outbounds, nil
	}
	for route, outboundRoute := range allOutbounds {
		outboundURL, err := mushroom.Parse(outboundRoute)
		if err != nil {
			return nil, fmt.Errorf("mushroom.Parse(%q): %w", outboundRoute, err)
		}
		if outboundURL.As(mushroom.SERVICE).AsDereference().String() != outboundServiceURL.As(mushroom.SERVICE).AsDereference().String() {
			if !excludeOutboundService {
				continue
			}
		} else if excludeOutboundService {
			continue
		}
		outbounds[route] = outboundURL
	}
	return outbounds, nil
}

// filterTopologyInbounds filters dep inbounds for routes whose inbound service matches inboundServiceURL.
// When excludeInboundService is true, matching routes are skipped instead of kept.
func filterTopologyInbounds(topologyInbounds map[string]map[string][]string, depServiceURL, inboundServiceURL mushroom.TopologyURL, excludeInboundService bool) (map[string]mushroom.TopologyURL, error) {
	// topologyInbounds is keyed by the service that owns the protected routes (this service).
	// Each entry lists remote routes allowed to call a local route; here we keep remote dep
	// routes that reach this service — the same edge as filterTopologyOutbounds, viewed from inbounds.
	allInbounds, hasInbounds := topologyInbounds[depServiceURL.As(mushroom.SERVICE).String()]
	if !hasInbounds {
		allInbounds, hasInbounds = topologyInbounds[inboundServiceURL.AsDereference().String()]
	}

	inbounds := make(map[string]mushroom.TopologyURL)
	if !hasInbounds || len(allInbounds) == 0 {
		return inbounds, nil
	}

	for route, inboundRoutes := range allInbounds {
		for _, inboundRoute := range inboundRoutes {
			inboundURL, err := mushroom.Parse(inboundRoute)
			if err != nil {
				return nil, fmt.Errorf("mushroom.Parse(%q): %w", inboundRoute, err)
			}
			match := inboundURL.Equal(inboundServiceURL, mushroom.SERVICE)
			if excludeInboundService {
				if match {
					continue
				}
			} else if !match {
				continue
			}
			inbounds[route] = inboundURL
		}
	}
	return inbounds, nil
}

func endpointForRouteURL(routeURL string, tp *topology.Client) (message.Endpoint, error) {
	routeMushroomURL, err := mushroom.Parse(routeURL)
	if err != nil {
		return message.Endpoint{}, fmt.Errorf("mushroom.Parse(%q): %w", routeURL, err)
	}

	service, err := tp.Service(routeMushroomURL.As(mushroom.SERVICE).AsDereference().String())
	if err != nil {
		return message.Endpoint{}, fmt.Errorf("topology.Service: %w", err)
	}

	handler, err := service.HandlerByCategory(routeMushroomURL.HandlerLink().HandlerCategory())
	if err != nil {
		return message.Endpoint{}, fmt.Errorf("HandlerByCategory(%q): %w", routeMushroomURL.HandlerLink().HandlerCategory(), err)
	}
	ind, ok := handler.AsIndependentHandler()
	if !ok {
		return message.Endpoint{}, fmt.Errorf("route %q is not an independent handler", routeURL)
	}

	return ind.Endpoint, nil
}

func buildTopologyOutbounds(
	topologyInbounds map[string]map[string][]string,
	depDerefs map[string]struct{},
	selfServiceDeref string,
) (map[string]map[string]string, error) {
	outbounds := make(map[string]map[string]string, len(depDerefs)+1)
	for handlerDeref := range depDerefs {
		handlerURL, err := mushroom.Parse(handlerDeref)
		if err != nil {
			return nil, fmt.Errorf("mushroom.Parse(%q): %w", handlerDeref, err)
		}
		serviceLink := handlerURL.As(mushroom.SERVICE).AsDereference().String()
		outbounds[serviceLink] = make(map[string]string)
	}

	for _, routeInbounds := range topologyInbounds {
		for route, inboundRoutes := range routeInbounds {
			for _, inboundRoute := range inboundRoutes {
				inboundURL, err := mushroom.Parse(inboundRoute)
				if err != nil {
					return nil, fmt.Errorf("mushroom.Parse(%q): %w", inboundRoute, err)
				}
				outboundDeref := inboundURL.As(mushroom.SERVICE).AsDereference().String()
				if _, ok := outbounds[outboundDeref]; !ok {
					if outboundDeref == selfServiceDeref {
						outbounds[selfServiceDeref] = make(map[string]string)
					} else {
						return nil, fmt.Errorf("outbound deref %q is not whitelisted", outboundDeref)
					}
				}
				outbounds[outboundDeref][inboundRoute] = route
			}
		}
	}

	return outbounds, nil
}
