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
	syncReplier "github.com/noPerfection/protocol/handler/sync_replier"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/topology"
)

const (
	IsServiceRunning = topology.IsServiceRunning
	StartService     = topology.StartService
	StopService      = topology.StopService
)

var _ topology.NodeInterface = (*Manager)(nil)

// The Manager keeps all necessary parameters of the service.
// Manage this service from other parts.
type Manager struct {
	base.Interface
	serviceName     string
	handlerControls []*clientSyncReplier.BaseControl
	topologyClient  *topology.Client
	blocker         **sync.WaitGroup
	running         bool
}

// New service with the parameters.
func New(serviceName string, managerEndpoint message.Endpoint) (*Manager, error) {
	topologyClient, err := topology.NewClient()
	if err != nil {
		return nil, fmt.Errorf("topology.NewClient: %w", err)
	}

	handler := syncReplier.New()

	h := &Manager{
		Interface:       handler,
		handlerControls: make([]*clientSyncReplier.BaseControl, 0),
		topologyClient:  topologyClient,
		serviceName:     serviceName,
	}

	managerConfig := HandlerConfig(serviceName, managerEndpoint)
	handler.SetConfig(managerConfig)

	return h, nil
}

func (m *Manager) SetSharedBlocker(blocker **sync.WaitGroup) {
	m.blocker = blocker
}

func (m *Manager) StartService(serviceName string, optionalParent ...*topology.ParentClient) (string, error) {
	if serviceName == "" || serviceName == m.serviceName {
		return strconv.Itoa(os.Getpid()), nil
	}

	return "", fmt.Errorf("service name is not empty and not equal to the service name")
}

func (m *Manager) IsServiceRunning(serviceName string) (bool, error) {
	if serviceName == "" || serviceName == m.serviceName {
		return m.running, nil
	}

	return false, fmt.Errorf("service name is not empty and not equal to the service name")
}

func (m *Manager) StopService(serviceName string) error {
	if serviceName != "" && serviceName != m.serviceName {
		return fmt.Errorf("service name is not empty and not equal to the service name")
	}

	if m.topologyClient != nil {
		if err := m.topologyClient.Close(); err != nil {
			return fmt.Errorf("topologyClient.Close: %w", err)
		}
		m.topologyClient = nil
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

	var optionalParent []*topology.ParentClient
	if kv, err := req.RouteParameters().NestedValue("parent"); err == nil {
		var parent topology.ParentClient
		if err := kv.Interface(&parent); err != nil {
			return req.Fail(fmt.Sprintf("kv.Interface('parent'): %v", err))
		}
		optionalParent = append(optionalParent, &parent)
	}

	id, err := m.StartService(serviceName, optionalParent...)
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

	go m.stopAfterReply(serviceName)

	return req.Ok(datatype.New())
}

func (m *Manager) stopAfterReply(serviceName string) {
	time.Sleep(100 * time.Millisecond)
	_ = m.StopService(serviceName)
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
	if m.topologyClient == nil {
		return fmt.Errorf("topologyClient is nil")
	}

	service, err := m.topologyClient.Service(m.serviceName)
	if err != nil {
		return fmt.Errorf("topologyClient.Service('%s'): %w", m.serviceName, err)
	}

	m.handlerControls = make([]*clientSyncReplier.BaseControl, 0, len(service.Handlers))
	for _, handler := range service.Handlers {
		if handler.Category == topology.ServiceManagerCategory {
			continue
		}

		control, err := clientSyncReplier.NewBaseControl(handler.Endpoint.Id+"_control", 0)
		if err != nil {
			return fmt.Errorf("sync_replier.NewBaseControl('%s_control'): %w", handler.Endpoint.Id, err)
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

	if err := m.setHandlerControls(); err != nil {
		return fmt.Errorf("setHandlerControls: %w", err)
	}

	if err := m.Interface.Start(); err != nil {
		return fmt.Errorf("handler.Start: %w", err)
	}

	m.running = true

	return nil
}
