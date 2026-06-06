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
	handlerControl "github.com/noPerfection/protocol/handler/control"
	syncReplier "github.com/noPerfection/protocol/handler/sync_replier"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/topology"
)

var _ topology.NodeInterface = (*ProxyManager)(nil)

// ProxyManager keeps all necessary parameters of the proxy service.
// Manage this proxy service from other parts.
type ProxyManager struct {
	base.Interface
	serviceName     string
	handlerControls []*clientSyncReplier.BaseControl
	topologyClient  *topology.Client
	blocker         **sync.WaitGroup
	running         bool
}

// NewProxyManager creates a manager for a proxy service.
func NewProxyManager(serviceName string, managerEndpoints ...message.Endpoint) (*ProxyManager, error) {
	if serviceName == "" {
		return nil, fmt.Errorf("serviceName is required")
	}
	if len(managerEndpoints) > 1 {
		return nil, fmt.Errorf("too many manager endpoints")
	}

	managerEndpoint := message.NewEndpoint(serviceName+"_proxy_"+topology.ServiceManagerCategory, 0)
	if len(managerEndpoints) == 1 {
		managerEndpoint = managerEndpoints[0]
	}
	topologyClient, err := topology.NewClient()
	if err != nil {
		return nil, fmt.Errorf("topology.NewClient: %w", err)
	}

	handler := syncReplier.New()

	h := &ProxyManager{
		Interface:       handler,
		handlerControls: make([]*clientSyncReplier.BaseControl, 0),
		topologyClient:  topologyClient,
		serviceName:     serviceName,
	}

	managerConfig := HandlerConfig(serviceName, managerEndpoint)
	handler.SetConfig(managerConfig)

	return h, nil
}

func (m *ProxyManager) SetSharedBlocker(blocker **sync.WaitGroup) {
	m.blocker = blocker
}

func (m *ProxyManager) StartService(serviceName string) (string, error) {
	if serviceName == "" || serviceName == m.serviceName {
		return strconv.Itoa(os.Getpid()), nil
	}

	return "", fmt.Errorf("service name is not empty and not equal to the service name")
}

func (m *ProxyManager) IsServiceRunning(serviceName string) (bool, error) {
	if serviceName == "" || serviceName == m.serviceName {
		return m.running, nil
	}

	return false, fmt.Errorf("service name is not empty and not equal to the service name")
}

func (m *ProxyManager) StopService(serviceName string) error {
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

// Close closes the manager, and service as well.
func (m *ProxyManager) Close() error {
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

func (m *ProxyManager) Running() bool {
	return m.running
}

func (m *ProxyManager) onIsServiceRunning(req message.RequestInterface) message.ReplyInterface {
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

func (m *ProxyManager) onStartService(req message.RequestInterface) message.ReplyInterface {
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

func (m *ProxyManager) onStopService(req message.RequestInterface) message.ReplyInterface {
	serviceName, err := req.RouteParameters().StringValue("service")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('service'): %v", err))
	}

	go m.stopAfterReply(serviceName)

	return req.Ok(datatype.New())
}

func (m *ProxyManager) stopAfterReply(serviceName string) {
	time.Sleep(100 * time.Millisecond)
	_ = m.StopService(serviceName)
}

func (m *ProxyManager) setHandlerControls() error {
	if m.topologyClient == nil {
		return fmt.Errorf("topologyClient is nil")
	}

	service, err := m.topologyClient.Service(m.serviceName)
	if err != nil {
		return fmt.Errorf("topologyClient.Service('%s'): %w", m.serviceName, err)
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

// Start the orchestra in the background.
// If it failed to run, then return an error.
// The url request is the main service to which this orchestra belongs too.
func (m *ProxyManager) Start() error {
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
