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
	syncReplier "github.com/noPerfection/protocol/handler/sync_replier"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service/handlers"
	"github.com/noPerfection/topology"
)

var _ topology.NodeInterface = (*ProxyManager)(nil)

// ProxyManager keeps all necessary parameters of the proxy service.
// Manage this proxy service from other parts.
type ProxyManager struct {
	base.Interface
	serviceName         string
	topologyClient      *topology.Client
	proxyHandlersClient *clientSyncReplier.Client
	blocker             **sync.WaitGroup
	running             bool
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

	proxyHandlersClient, err := clientSyncReplier.NewClient(serviceName+handlers.ProxyManagerCategory, 0)
	if err != nil {
		_ = topologyClient.Close()
		return nil, fmt.Errorf("sync_replier.NewClient('%s'): %w", serviceName+handlers.ProxyManagerCategory, err)
	}

	handler := syncReplier.New()

	h := &ProxyManager{
		Interface:           handler,
		topologyClient:      topologyClient,
		proxyHandlersClient: proxyHandlersClient,
		serviceName:         serviceName,
	}

	managerConfig := HandlerConfig(serviceName, managerEndpoint)
	handler.SetConfig(managerConfig)

	return h, nil
}

func (m *ProxyManager) SetSharedBlocker(blocker **sync.WaitGroup) {
	m.blocker = blocker
}

// For now, let's make it not starting. It just returns its own name.
// Later it will just keep almost identical to Start() data.
func (m *ProxyManager) StartService(serviceName string) (string, error) {
	if serviceName != "" && serviceName != m.serviceName {
		return "", fmt.Errorf("service name is not empty and not equal to the service name")
	}

	if err := m.ensureTopologyClient(); err != nil {
		return "", err
	}
	if err := m.ensureProxyHandlersClient(); err != nil {
		return "", err
	}
	if err := m.proxyHandlersRequest(handlers.StartProxyHandlersCommand); err != nil {
		return "", err
	}

	m.running = true
	return strconv.Itoa(os.Getpid()), nil
}

// For now, lets just return manager.running.
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

	if m.proxyHandlersClient != nil {
		if err := m.proxyHandlersRequest(handlers.StopProxyHandlersCommand); err != nil {
			return err
		}
	}
	if m.proxyHandlersClient != nil {
		if err := m.proxyHandlersClient.Close(); err != nil {
			return fmt.Errorf("proxyHandlersClient.Close: %w", err)
		}
		m.proxyHandlersClient = nil
	}
	if m.topologyClient != nil {
		if err := m.topologyClient.Close(); err != nil {
			return fmt.Errorf("topologyClient.Close: %w", err)
		}
		m.topologyClient = nil
	}

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

func (m *ProxyManager) ensureTopologyClient() error {
	if m.topologyClient != nil {
		return nil
	}
	topologyClient, err := topology.NewClient()
	if err != nil {
		return fmt.Errorf("topology.NewClient: %w", err)
	}
	m.topologyClient = topologyClient
	return nil
}

func (m *ProxyManager) ensureProxyHandlersClient() error {
	if m.proxyHandlersClient != nil {
		return nil
	}
	proxyHandlersClient, err := clientSyncReplier.NewClient(m.serviceName+handlers.ProxyManagerCategory, 0)
	if err != nil {
		return fmt.Errorf("sync_replier.NewClient('%s'): %w", m.serviceName+handlers.ProxyManagerCategory, err)
	}
	m.proxyHandlersClient = proxyHandlersClient
	return nil
}

func (m *ProxyManager) proxyHandlersRequest(command string) error {
	if err := m.ensureProxyHandlersClient(); err != nil {
		return err
	}
	reply, err := m.proxyHandlersClient.Request(&message.Request{
		Command:    command,
		Parameters: datatype.New(),
	})
	if err != nil {
		return fmt.Errorf("proxyHandlersClient.Request('%s'): %w", command, err)
	}
	if !reply.IsOK() {
		return fmt.Errorf("proxyHandlersClient.Request('%s'): %s", command, reply.ErrorMessage())
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

	if m.proxyHandlersClient == nil {
		return fmt.Errorf("proxyHandlersClient is nil")
	}

	if err := m.Interface.Start(); err != nil {
		return fmt.Errorf("handler.Start: %w", err)
	}

	m.running = true

	return nil
}
