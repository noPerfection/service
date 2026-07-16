// Package manager is the manager of the service.
package manager

import (
	"fmt"
	"sync"
	"time"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/protocol/client"
	protocolHandler "github.com/noPerfection/protocol/handler"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service/handlers"
	"github.com/noPerfection/service/mushroom"
	"github.com/noPerfection/topology"
	"github.com/noPerfection/topology/config"
)

const (
	IsServiceRunning          = topology.IsServiceRunning
	StartService              = topology.StartService
	StopService               = topology.StopService
	Services                  = topology.Services
	InprocTopologyServiceName = "inproc-topology"

	// ManagerPublicKeyParam is the service Parameters key under which the manager's
	// CURVE public key is stored by allowServiceManager.
	ManagerPublicKeyParam = "public-key"
)

// DefaultExtensionManagerEndpoint returns the default endpoint for a service's extension manager.
func DefaultExtensionManagerEndpoint(serviceName string) message.Endpoint {
	return message.NewEndpoint(serviceName+"_ext_"+config.ServiceManagerCategory, 0)
}

var _ topology.NodeInterface = (*Manager)(nil)

// The Manager keeps all necessary parameters of the service.
// Manage this service from other parts.
type Manager struct {
	protocolHandler.Interface
	serviceURL      mushroom.TopologyURL // mushroomURL of this service in the topology mycelium
	handlerControls []*client.Control
	topology        *topology.Client
	blocker         **sync.WaitGroup
	started         bool
	running         bool
	secretKey       string
	pubKey          string
}

// New creates a manager for an independent service.
// serviceURL is the mushroomURL used to locate this service in the topology mycelium
// (a plain symbol such as "main", or a full dereference URL).
// managerEndpoint is the socket other processes use to start, stop, and probe this service.
// New creates a manager for an independent service.
// An optional secretKey may be provided; if given, the public key is derived from it.
// If omitted, a fresh CURVE keypair is generated.
func New(serviceURL mushroom.TopologyURL, managerEndpoint message.Endpoint, secretKey ...string) (*Manager, error) {
	topology, err := topology.NewClient()
	if err != nil {
		return nil, fmt.Errorf("topology.NewClient: %w", err)
	}

	var pub, sec string
	if len(secretKey) > 0 && secretKey[0] != "" {
		sec = secretKey[0]
		pub, err = message.DerivePublicKey(sec)
		if err != nil {
			return nil, fmt.Errorf("message.DerivePublicKey: %w", err)
		}
	} else {
		pub, sec, err = message.GenerateCurveKey()
		if err != nil {
			return nil, fmt.Errorf("message.GenerateCurveKey: %w", err)
		}
	}
	topology.Secure(sec)
	fmt.Printf("Generated CURVE key pair for manager %s: pubKey=%s\n", serviceURL.String(), pub)

	handler := protocolHandler.NewReplier()

	h := &Manager{
		Interface:       handler,
		handlerControls: make([]*client.Control, 0),
		topology:        topology,
		serviceURL:      serviceURL,
		secretKey:       sec,
		pubKey:          pub,
	}

	handler.SetEndpoint(managerEndpoint)

	return h, nil
}

// PublicKey returns the CURVE public key for this manager's handler.
func (m *Manager) PublicKey() string {
	return m.pubKey
}

func (m *Manager) SetSharedBlocker(blocker **sync.WaitGroup) {
	m.blocker = blocker
}

func (m *Manager) selfService() (config.Service, error) {
	if m.topology == nil {
		return config.Service{}, fmt.Errorf("topology is nil")
	}
	return m.topology.Service(m.serviceURL.AsDereference().String())
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
		return "", fmt.Errorf("can't start itself: service is already running")
	}
	if m.topology == nil {
		return "", fmt.Errorf("topology is nil")
	}
	record, err := m.topology.Service(serviceURL)
	if err == nil && record.IsInproc() {
		endpoint, handlerType, err := inprocTopologyExtensionEndpoint(m.topology)
		if err != nil {
			return "", err
		}
		return startInprocService(endpoint, handlerType, record.Name)
	}
	return m.topology.StartService(serviceURL)
}

func (m *Manager) IsServiceRunning(serviceURL string, attempts ...int) (bool, error) {
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
	return isServiceRunningWithReload(m.topology, serviceURL, m.secretKey, attempts...)
}

func (m *Manager) TestSecretKey() string {
	return m.secretKey
}

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

// Stops and unlocks the blocker of the service.
// If the service is watched using service.Watch(), then it will be unlocked.
// To start this service, call the Start() method. Or StartService from its parent.
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
		if err := stopRemoteService(m.topology, serviceURL, m.secretKey); err != nil {
			if localErr := m.topology.StopService(serviceURL); localErr == nil {
				return nil
			}
			return fmt.Errorf("stopRemoteService(%q): %w", serviceURL, err)
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
	m.handlerControls = make([]*client.Control, 0)

	wasRunning := m.running
	m.running = false
	if wasRunning && m.blocker != nil && *m.blocker != nil {
		(*m.blocker).Done()
	}

	return nil
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

// Close closes the manager, and service as well.
func (m *Manager) Close() error {
	if m == nil {
		return fmt.Errorf("manager is nil")
	}

	if err := m.StopService(m.serviceURL.AsDereference().String()); err != nil {
		return err
	}
	if err := handlers.CloseViaControl(m.Interface); err != nil {
		return fmt.Errorf("manager handler close: %w", err)
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

func (m *Manager) setHandlerControls() error {
	if m.topology == nil {
		return fmt.Errorf("topology is nil")
	}

	service, err := m.selfService()
	if err != nil {
		return fmt.Errorf("topology.Service(%q): %w", m.serviceURL, err)
	}

	m.handlerControls = make([]*client.Control, 0, len(service.Handlers))
	for _, handlerVariant := range service.Handlers {
		handler, ok := handlerVariant.AsIndependentHandler()
		if !ok {
			continue
		}
		if handler.Category == config.ServiceManagerCategory {
			continue
		}

		controlEndpoint := protocolHandler.NewInternalControlEndpoint(handler.Endpoint)
		control, err := client.NewControl(controlEndpoint.Id, controlEndpoint.Port)
		if err != nil {
			return fmt.Errorf("client.NewControl('%s'): %w", controlEndpoint.Id, err)
		}
		m.handlerControls = append(m.handlerControls, control)
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

	handlerLink := m.serviceURL.New(config.ServiceManagerCategory)
	m.Interface.SetMushroomURL(handlerLink.String())

	m.Interface.Secure(m.secretKey)

	if err := m.Interface.Start(); err != nil {
		return fmt.Errorf("handler.Start: %w", err)
	}

	m.started = true
	m.running = true

	return nil
}
