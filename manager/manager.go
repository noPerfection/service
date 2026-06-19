// Package manager is the manager of the service.
package manager

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

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
	serviceURL      string // mushroomURL of this service in the topology mycelium
	handlerControls []*clientSyncReplier.BaseControl
	topology        *topology.Client
	blocker         **sync.WaitGroup
	started         bool
	running         bool
}

// New creates a manager for an independent service.
// serviceURL is the mushroomURL used to locate this service in the topology mycelium
// (a plain symbol such as "main", or a full dereference URL).
// managerEndpoint is the socket other processes use to start, stop, and probe this service.
func New(serviceURL string, managerEndpoint message.Endpoint) (*Manager, error) {
	topology, err := topology.NewClient()
	if err != nil {
		return nil, fmt.Errorf("topology.NewClient: %w", err)
	}

	handler := syncReplier.New()

	h := &Manager{
		Interface:       handler,
		handlerControls: make([]*clientSyncReplier.BaseControl, 0),
		topology:        topology,
		serviceURL:      serviceURL,
	}

	managerConfig := HandlerConfig(managerEndpoint)
	handler.SetConfig(managerConfig)

	return h, nil
}

func (m *Manager) SetSharedBlocker(blocker **sync.WaitGroup) {
	m.blocker = blocker
}

func (m *Manager) selfService() (config.Service, error) {
	if m.topology == nil {
		return config.Service{}, fmt.Errorf("topology is nil")
	}
	return m.topology.Service(m.serviceURL)
}

// matchesSelf reports whether serviceURL refers to this manager's service.
// Empty serviceURL means this process. Both URLs are resolved through topology
// and compared with config.Service.Equal (name and manager endpoint).
func (m *Manager) matchesSelf(serviceURL string) (bool, error) {
	if serviceURL == "" {
		return true, nil
	}
	if m.topology == nil {
		return false, fmt.Errorf("topology is nil")
	}
	self, err := m.selfService()
	if err != nil {
		return false, err
	}
	other, err := m.topology.Service(serviceURL)
	if err != nil {
		return false, err
	}
	return self.Equal(other), nil
}

func (m *Manager) StartService(serviceURL string) (string, error) {
	match, err := m.matchesSelf(serviceURL)
	if err != nil {
		return "", err
	}
	if match {
		return strconv.Itoa(os.Getpid()), nil
	}
	if m.topology == nil {
		return "", fmt.Errorf("topology is nil")
	}
	return m.topology.StartService(serviceURL)
}

func (m *Manager) StartServiceByConfig(record config.Service) (string, error) {
	self, err := m.selfService()
	if err != nil {
		return "", err
	}
	if record.Equal(self) {
		return strconv.Itoa(os.Getpid()), nil
	}

	managerHandler, err := record.HandlerByCategory(topology.ServiceManagerCategory)
	if err != nil {
		if m.topology == nil {
			return "", fmt.Errorf("topology is nil")
		}
		return m.topology.StartService(record.Name)
	}
	handler, ok := managerHandler.AsIndependentHandler()
	if !ok {
		return "", fmt.Errorf("service %q manager handler is not independent", record.Name)
	}
	id, err := m.startServiceOnManager(record.Name, handler.Endpoint)
	if err != nil {
		if m.topology == nil {
			return "", err
		}
		return m.topology.StartService(record.Name)
	}
	return id, nil
}

func (m *Manager) IsServiceRunning(serviceURL string) (bool, error) {
	match, err := m.matchesSelf(serviceURL)
	if err != nil {
		return false, err
	}
	if match {
		return m.running, nil
	}
	if m.topology == nil {
		return false, fmt.Errorf("topology is nil")
	}
	return m.topology.IsServiceRunning(serviceURL)
}

func (m *Manager) IsServiceRunningByManager(serviceURL string, handler config.IndependentHandler) (bool, error) {
	match, err := m.matchesSelf(serviceURL)
	if err != nil {
		return false, err
	}
	if match {
		return m.running, nil
	}
	return m.isServiceRunningOnManager(serviceURL, handler.Endpoint)
}

func (m *Manager) isServiceRunningOnManager(serviceName string, endpoint message.Endpoint) (bool, error) {
	client, err := clientSyncReplier.NewClient(endpoint.Id, endpoint.Port)
	if err != nil {
		return false, fmt.Errorf("sync_replier.NewClient: %w", err)
	}
	defer client.Close()

	client.Timeout(100 * time.Millisecond)
	client.Attempt(2)

	reply, err := client.Request(&message.Request{
		Command:    IsServiceRunning,
		Parameters: datatype.New().Set("service", serviceName),
	})
	if err != nil {
		return false, nil
	}
	if !reply.IsOK() {
		return false, fmt.Errorf("reply.Message: %s", reply.ErrorMessage())
	}

	running, err := reply.ReplyParameters().BoolValue("running")
	if err != nil {
		return false, fmt.Errorf("reply.Parameters.GetBoolean('running'): %w", err)
	}
	return running, nil
}

func (m *Manager) startServiceOnManager(serviceName string, endpoint message.Endpoint) (string, error) {
	client, err := clientSyncReplier.NewClient(endpoint.Id, endpoint.Port)
	if err != nil {
		return "", fmt.Errorf("sync_replier.NewClient: %w", err)
	}
	defer client.Close()

	client.Timeout(time.Second)
	client.Attempt(3)

	reply, err := client.Request(&message.Request{
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

func (m *Manager) StopService(serviceURL string) error {
	if !m.running && m.started {
		return nil
	}

	match, err := m.matchesSelf(serviceURL)
	if err != nil {
		return err
	}
	if serviceURL != "" && !match {
		if m.topology == nil {
			return fmt.Errorf("topology is nil")
		}
		return m.topology.StopService(serviceURL)
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

	if err := m.StopService(m.serviceURL); err != nil {
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
func HandlerConfig(managerEndpoint message.Endpoint) *handlerConfig.Handler {
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

	service, err := m.selfService()
	if err != nil {
		return fmt.Errorf("topology.Service(%q): %w", m.serviceURL, err)
	}

	m.handlerControls = make([]*clientSyncReplier.BaseControl, 0, len(service.Handlers))
	for _, handlerVariant := range service.Handlers {
		handler, ok := handlerVariant.AsIndependentHandler()
		if !ok {
			continue
		}
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

// Start registers manager routes and connects handler controls for this service.
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

	m.started = true
	m.running = true

	return nil
}
