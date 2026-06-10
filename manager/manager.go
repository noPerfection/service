// Package manager is the manager of the service.
package manager

import (
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/noPerfection/datatype"
	clientSyncReplier "github.com/noPerfection/protocol/client/sync_replier"
	"github.com/noPerfection/protocol/handler/base"
	handlerConfig "github.com/noPerfection/protocol/handler/config"
	handlerControl "github.com/noPerfection/protocol/handler/control"
	syncReplier "github.com/noPerfection/protocol/handler/sync_replier"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/topology"
	"github.com/noPerfection/topology/config"
)

const (
	IsServiceRunning = topology.IsServiceRunning
	StartService     = topology.StartService
	StopService      = topology.StopService
	Services         = topology.Services
)

var _ topology.NodeInterface = (*Manager)(nil)

// The Manager keeps all necessary parameters of the service.
// Manage this service from other parts.
type Manager struct {
	base.Interface
	serviceName     string
	handlerControls []*clientSyncReplier.BaseControl
	topology        *topology.Client
	blocker         **sync.WaitGroup
	running         bool
}

// New service with the parameters.
func New(serviceName string, managerEndpoint message.Endpoint) (*Manager, error) {
	topology, err := topology.NewClient()
	if err != nil {
		return nil, fmt.Errorf("topology.NewClient: %w", err)
	}

	handler := syncReplier.New()

	h := &Manager{
		Interface:       handler,
		handlerControls: make([]*clientSyncReplier.BaseControl, 0),
		topology:        topology,
		serviceName:     serviceName,
	}

	managerConfig := HandlerConfig(serviceName, managerEndpoint)
	handler.SetConfig(managerConfig)

	return h, nil
}

func (m *Manager) SetSharedBlocker(blocker **sync.WaitGroup) {
	m.blocker = blocker
}

func (m *Manager) StartService(serviceName string) (string, error) {
	if serviceName == "" || serviceName == m.serviceName {
		return strconv.Itoa(os.Getpid()), nil
	}
	if m.topology == nil {
		return "", fmt.Errorf("topology is nil")
	}
	return m.topology.StartService(serviceName)
}

func (m *Manager) StartServiceByConfig(record config.Service) (string, error) {
	if record.Name == "" || record.Name == m.serviceName {
		return strconv.Itoa(os.Getpid()), nil
	}
	if m.topology == nil {
		return "", fmt.Errorf("topology is nil")
	}
	return m.topology.StartServiceByConfig(record)
}

func (m *Manager) IsServiceRunning(serviceName string) (bool, error) {
	if serviceName == "" || serviceName == m.serviceName {
		return m.running, nil
	}
	if m.topology == nil {
		return false, fmt.Errorf("topology is nil")
	}
	return m.topology.IsServiceRunning(serviceName)
}

func (m *Manager) IsServiceRunningByManager(serviceName string, handler config.Handler) (bool, error) {
	if serviceName == "" || serviceName == m.serviceName {
		return m.running, nil
	}
	if m.topology == nil {
		return false, fmt.Errorf("topology is nil")
	}
	return m.topology.IsServiceRunningByManager(serviceName, handler)
}

func (m *Manager) StopService(serviceName string) error {
	if serviceName != "" && serviceName != m.serviceName {
		if m.topology == nil {
			return fmt.Errorf("topology is nil")
		}
		return m.topology.StopService(serviceName)
	}

	if m.topology != nil {
		if err := m.topology.Close(); err != nil {
			return fmt.Errorf("topology.Close: %w", err)
		}
		m.topology = nil
	}
	for _, control := range m.handlerControls {
		if err := control.HandlerClose(); err != nil {
			return fmt.Errorf("handlerControl.HandlerClose: %w", err)
		}
		if err := control.Close(); err != nil {
			return fmt.Errorf("handlerControl.Close: %w", err)
		}
	}
	m.handlerControls = make([]*clientSyncReplier.BaseControl, 0)

	wasRunning := m.running
	m.running = false
	if wasRunning && m.blocker != nil && *m.blocker != nil {
		(*m.blocker).Done()
	}

	return nil
}

// Close closes the manager, and service as well.
func (m *Manager) Close() error {
	if m == nil {
		return fmt.Errorf("manager is nil")
	}

	if err := m.StopService(m.serviceName); err != nil {
		return err
	}
	if err := closeHandler(m.Interface); err != nil {
		return fmt.Errorf("manager handler close: %w", err)
	}
	if handler, ok := m.Interface.(*syncReplier.SyncReplier); ok {
		if err := closeHandler(handler.Control); err != nil {
			return fmt.Errorf("manager control close: %w", err)
		}
	}

	return nil
}

func (m *Manager) Running() bool {
	return m.running
}

func (m *Manager) onIsServiceRunning(req message.RequestInterface) message.ReplyInterface {
	serviceName, err := req.RouteParameters().StringValue("service")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('service'): %v", err))
	}

	running, err := m.IsServiceRunning(serviceName)
	if err != nil {
		return req.Fail(fmt.Sprintf("manager.IsServiceRunning('%s'): %v", serviceName, err))
	}

	return req.Ok(datatype.New().Set("running", running))
}

func (m *Manager) onStartService(req message.RequestInterface) message.ReplyInterface {
	serviceName, err := req.RouteParameters().StringValue("service")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('service'): %v", err))
	}

	id, err := m.StartService(serviceName)
	if err != nil {
		return req.Fail(fmt.Sprintf("manager.StartService('%s'): %v", serviceName, err))
	}

	return req.Ok(datatype.New().Set("id", id))
}

func (m *Manager) onStopService(req message.RequestInterface) message.ReplyInterface {
	serviceName, err := req.RouteParameters().StringValue("service")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('service'): %v", err))
	}

	if err := m.StopService(serviceName); err != nil {
		return req.Fail(fmt.Sprintf("manager.StopService('%s'): %v", serviceName, err))
	}

	return req.Ok(datatype.New())
}

func (m *Manager) onServices(req message.RequestInterface) message.ReplyInterface {
	if m.topology == nil {
		return req.Fail("topology is nil")
	}

	services, err := m.topology.Services()
	if err != nil {
		return req.Fail(fmt.Sprintf("topology.Services: %v", err))
	}

	return req.Ok(datatype.New().Set("services", services))
}

// HandlerConfig returns the manager handler configuration.
func HandlerConfig(serviceName string, managerEndpoint message.Endpoint) *handlerConfig.Handler {
	return handlerConfig.New(
		handlerConfig.SyncReplierType,
		managerEndpoint.Id,
		topology.ServiceManagerCategory,
		managerEndpoint.Port,
	)
}

func (m *Manager) setHandlerControls() error {
	if m.topology == nil {
		return fmt.Errorf("topology is nil")
	}

	service, err := m.topology.Service(m.serviceName)
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", m.serviceName, err)
	}

	m.handlerControls = make([]*clientSyncReplier.BaseControl, 0, len(service.Handlers))
	for _, handlerVariant := range service.Handlers {
		handler := handlerVariant.AsHandler()
		if handler.Category == topology.ServiceManagerCategory {
			continue
		}

		controlID := handlerControl.ControlEndpointID(handler.Endpoint.Id, handler.Endpoint.Port)
		control, err := clientSyncReplier.NewBaseControl(controlID, 0)
		if err != nil {
			return fmt.Errorf("sync_replier.NewBaseControl('%s'): %w", controlID, err)
		}
		m.handlerControls = append(m.handlerControls, control)
	}

	return nil
}

func closeHandler(handler base.Interface) error {
	if handler == nil {
		return nil
	}

	handler.SetClose(true)
	if socket := handler.Socket(); socket != nil {
		if err := socket.Close(); err != nil {
			return fmt.Errorf("handler(category: '%s').Socket.Close: %w", topology.ServiceManagerCategory, err)
		}
		handler.SetSocketNil()
	}

	return nil
}

// Start the orchestra in the background.
// If it failed to run, then return an error.
// The url request is the main service to which this orchestra belongs too.
func (m *Manager) Start() error {
	if err := m.Interface.Route(IsServiceRunning, m.onIsServiceRunning); err != nil {
		return fmt.Errorf(`handler.Route("%s"): %w`, IsServiceRunning, err)
	}
	if err := m.Interface.Route(StartService, m.onStartService); err != nil {
		return fmt.Errorf(`handler.Route("%s"): %w`, StartService, err)
	}
	if err := m.Interface.Route(StopService, m.onStopService); err != nil {
		return fmt.Errorf(`handler.Route("%s"): %w`, StopService, err)
	}
	if err := m.Interface.Route(Services, m.onServices); err != nil {
		return fmt.Errorf(`handler.Route("%s"): %w`, Services, err)
	}

	if err := m.setHandlerControls(); err != nil {
		return fmt.Errorf("setHandlerControls: %w", err)
	}

	if err := m.Interface.Start(); err != nil {
		return fmt.Errorf("handler.Start: %w", err)
	}

	m.running = true

	return nil
}
