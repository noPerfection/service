package manager

import (
	"fmt"
	"strings"

	protocolHandler "github.com/noPerfection/protocol/handler"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service/mushroom"
	"github.com/noPerfection/topology"
	"github.com/noPerfection/topology/config"
)

// AddTopologyManagers authorizes this manager's message.Any route to call every
// other service manager registered in npac. Call after outbound registration.
func (m *Manager) AddTopologyManagers() error {
	if m.topology == nil {
		return fmt.Errorf("topology is nil")
	}
	return addTopologyManagers(m.topology, m.serviceURL.AsDereference().String(), m.Interface)
}

// AddTopologyManagers authorizes this proxy manager's message.Any route to call
// every other service manager registered in npac. Call after outbound registration.
func (m *ProxyManager) AddTopologyManagers() error {
	if err := m.ensureTopologyClient(); err != nil {
		return err
	}
	return addTopologyManagers(m.topology, m.serviceName, m.Interface)
}

func addTopologyManagers(tp *topology.Client, serviceName string, handler protocolHandler.Interface) error {
	replier, err := handlerWithAutocontext(handler)
	if err != nil {
		return err
	}

	self, err := tp.Service(serviceName)
	if err != nil {
		return fmt.Errorf("topology.Service(%q): %w", serviceName, err)
	}

	services, err := tp.Services()
	if err != nil {
		return fmt.Errorf("topology.Services: %w", err)
	}

	for _, service := range services {
		if service.Equal(self) {
			continue
		}
		if _, err := service.HandlerByCategory(config.ServiceManagerCategory); err != nil {
			continue
		}

		serviceLink, err := tp.GetLink(service.Name)
		if err != nil {
			return fmt.Errorf("topology.GetLink(%q): %w", service.Name, err)
		}

		outbound, err := mushroom.New(serviceLink, config.ServiceManagerCategory, message.Any)
		if err != nil {
			return fmt.Errorf("mushroom.New(%q, %q, %q): %w", serviceLink, config.ServiceManagerCategory, message.Any, err)
		}

		if err := replier.NpacSecureEdgeCase(outbound.String(), message.Any); err != nil {
			if strings.Contains(err.Error(), "already whitelisted") {
				continue
			}
			return fmt.Errorf("NpacSecureEdgeCase(%q): %w", service.Name, err)
		}
	}

	return nil
}

func handlerWithAutocontext(handler protocolHandler.Interface) (*protocolHandler.Replier, error) {
	replier, ok := handler.(*protocolHandler.Replier)
	if !ok {
		return nil, fmt.Errorf("manager handler is not a replier")
	}
	if replier.Autocontext == nil {
		return nil, fmt.Errorf("manager handler autocontext is nil")
	}
	return replier, nil
}
