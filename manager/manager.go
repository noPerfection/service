// Package manager is the manager of the service.
package manager

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/log"
	"github.com/noPerfection/protocol/client"
	protocolClient "github.com/noPerfection/protocol/client"
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

	defaultHandshakeInterval = 5 * time.Second

	// Handshake is the manager route used to exchange command whitelist secrets.
	// It is intentionally excluded from RequireWhitelist enforcement.
	Handshake = "handshake"

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
	serviceURL        mushroom.TopologyURL // mushroomURL of this service in the topology mycelium
	handlerControls   []*client.Control
	topology          *topology.Client
	blocker           **sync.WaitGroup
	started           bool
	running           bool
	handshaked        bool
	secretKey         string
	pubKey            string
	mu                sync.Mutex
	inbounds          map[string]string // caller manager link -> HMAC secret
	outbounds         map[string]string // dep service dereference URL -> HMAC secret
	selfServiceName   string
	logger            *log.Logger
	handshakeStop     chan struct{}
	handshakeDone     sync.WaitGroup
	handshakeInterval time.Duration
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
		handshaked:      false,
		pubKey:          pub,
		inbounds:        make(map[string]string),
		outbounds:       make(map[string]string),
	}

	handler.SetEndpoint(managerEndpoint)

	return h, nil
}

// PublicKey returns the CURVE public key for this manager's handler.
func (m *Manager) PublicKey() string {
	return m.pubKey
}

func managerWhitelistCommands() []string {
	return []string{
		IsServiceRunning,
		StartService,
		StopService,
		Services,
	}
}

// RequireWhitelist marks every manager route except handshake as requiring a whitelist entry.
func (m *Manager) RequireWhitelist() {
	for _, cmd := range managerWhitelistCommands() {
		m.Interface.RequireWhitelist(cmd)
	}
}

// SetLogger sets the optional logger for this manager.
func (m *Manager) SetLogger(logger *log.Logger) error {
	m.logger = logger
	if err := m.Interface.SetLogger(logger); err != nil {
		return fmt.Errorf("manager SetLogger: %w", err)
	}
	return nil
}

// SetHandshakeInterval overrides the background handshake period.
// Zero restores the default interval. Negative disables the background loop.
// Must be called before Start.
func (m *Manager) SetHandshakeInterval(interval time.Duration) {
	m.handshakeInterval = interval
}

func (m *Manager) handshakePeriod() time.Duration {
	if m.handshakeInterval > 0 {
		return m.handshakeInterval
	}
	return defaultHandshakeInterval
}

func (m *Manager) startBackgroundHandshake() {
	if m.handshakeInterval < 0 {
		return
	}
	stopCh := make(chan struct{})
	m.handshakeStop = stopCh
	m.handshakeDone.Go(func() {
		ticker := time.NewTicker(m.handshakePeriod())
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				if err := m.Handshake(); err != nil && m.logger != nil {
					m.logger.Warn("background handshake failed", "error", err)
				}
			}
		}
	})
}

func (m *Manager) stopBackgroundHandshake() {
	if m.handshakeStop == nil {
		return
	}
	close(m.handshakeStop)
	m.handshakeDone.Wait()
	m.handshakeStop = nil
}

func (m *Manager) SetSharedBlocker(blocker **sync.WaitGroup) {
	m.blocker = blocker
}

func outboundDereferenceKey(serviceURL string, tp *topology.Client) (string, error) {
	link := serviceURL
	if tp != nil {
		if resolved, err := tp.GetLink(serviceURL); err == nil {
			link = resolved
		}
	}

	u, err := mushroom.Parse(link)
	if err != nil {
		u, err = mushroom.New(link)
		if err != nil {
			return "", fmt.Errorf("mushroom.Parse/New(%q): %w", serviceURL, err)
		}
	}

	return u.As(mushroom.SERVICE).AsDereference().String(), nil
}

func (m *Manager) getHmacSecret(serviceURL string) string {
	key, err := outboundDereferenceKey(serviceURL, m.topology)
	if err != nil {
		return ""
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	return m.outbounds[key]
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
	if m.selfServiceName != "" && serviceURL == m.selfServiceName {
		return true, nil
	}
	if m.topology == nil {
		return false, fmt.Errorf("topology is nil")
	}

	selfLink, err := m.topology.GetLink(m.serviceURL.AsDereference().String())
	if err != nil {
		return false, err
	}
	otherLink, err := m.topology.GetLink(serviceURL)
	if err != nil {
		return false, err
	}
	return selfLink == otherLink, nil
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
	return isServiceRunningWithReload(m.topology, serviceURL, m.secretKey, m.getHmacSecret(serviceURL), attempts...)
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
		if err := stopRemoteService(m.topology, serviceURL, m.secretKey, m.getHmacSecret(serviceURL)); err != nil {
			if localErr := m.topology.StopService(serviceURL); localErr == nil {
				return nil
			}
			return fmt.Errorf("stopRemoteService(%q): %w", serviceURL, err)
		}
		return m.topology.StopService(serviceURL)
	}

	m.stopBackgroundHandshake()

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

	fmt.Println("---------onIsServiceRunning", serviceName)

	if serviceName == m.selfServiceName || serviceName == "" {
		return req.Ok(datatype.New().Set("running", m.running))
	}

	match, err := m.matchesSelf(serviceName)
	if err != nil {
		return req.Fail(fmt.Sprintf("manager.matchesSelf('%s'): %v", serviceName, err))
	}
	if match {
		return req.Ok(datatype.New().Set("running", m.running))
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

func (m *Manager) onHandshake(req message.RequestInterface) message.ReplyInterface {
	secret, err := req.RouteParameters().StringValue("secret")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('secret'): %v", err))
	}
	if secret == "" {
		return req.Fail("secret is required")
	}

	inboundURL, err := req.RouteParameters().StringValue("inbound-url")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('inbound-url'): %v", err))
	}
	if inboundURL == "" {
		return req.Fail("inbound-url is required")
	}

	m.mu.Lock()
	if _, ok := m.outbounds[inboundURL]; ok {
		m.mu.Unlock()
		return req.Fail("inbound-url conflicts with outbound")
	}
	if _, ok := m.inbounds[inboundURL]; ok {
		m.mu.Unlock()
		return req.Ok(datatype.New())
	}
	m.inbounds[inboundURL] = secret
	m.mu.Unlock()

	for _, cmd := range managerWhitelistCommands() {
		if err := m.Interface.Whitelist(cmd, secret); err != nil {
			return req.Fail(fmt.Sprintf(`handler.Whitelist("%s"): %v`, cmd, err))
		}
	}

	return req.Ok(datatype.New())
}

func (m *Manager) handshakeOutbound(depURL string) error {
	secret := m.secretKey

	key, err := outboundDereferenceKey(depURL, m.topology)
	if err != nil {
		return fmt.Errorf("outboundDereferenceKey(%q): %w", depURL, err)
	}

	m.mu.Lock()
	m.outbounds[key] = secret
	m.mu.Unlock()

	serviceLink, err := m.topology.GetLink(m.serviceURL.AsDereference().String())
	if err != nil {
		return fmt.Errorf("topology.GetLink(%q): %w", m.serviceURL.AsDereference().String(), err)
	}
	managerLink, err := mushroom.New(serviceLink, config.ServiceManagerCategory)
	if err != nil {
		return fmt.Errorf("mushroom.New(%q, %q): %w", serviceLink, config.ServiceManagerCategory, err)
	}
	inboundURL := managerLink.String()

	depService, err := m.topology.Service(depURL)
	if err != nil {
		return fmt.Errorf("topology.Service(%q): %w", depURL, err)
	}

	managerHandler, err := depService.HandlerByCategory(config.ServiceManagerCategory)
	if err != nil {
		return fmt.Errorf("dep %q manager handler: %w", depService.Name, err)
	}
	ind, ok := managerHandler.AsIndependentHandler()
	if !ok {
		return fmt.Errorf("dep %q manager handler is invalid", depService.Name)
	}

	socket, err := protocolClient.New(ind.Endpoint.Id, ind.Endpoint.Port, protocolClient.SyncReplierType)
	if err != nil {
		return fmt.Errorf("client.New: %w", err)
	}
	defer socket.Close()

	node := &topology.Client{Socket: socket}
	if depService.Parameters != nil {
		if pubKey, ok := depService.Parameters[ManagerPublicKeyParam].(string); ok && pubKey != "" {
			node.Socket.Allow(pubKey)
		}
	}
	node.Socket.Secure(m.secretKey)
	node.Timeout(managerProbeTimeout(depService))
	node.Attempt(2)

	reply, err := node.Request(&message.Request{
		Command: Handshake,
		Parameters: datatype.New().
			Set("secret", secret).
			Set("inbound-url", inboundURL),
	})
	if err != nil {
		return fmt.Errorf("socket.Request(%q): %w", Handshake, err)
	}
	if !reply.IsOK() {
		return fmt.Errorf("reply.Message: %s", reply.ErrorMessage())
	}

	return nil
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

func (m *Manager) getDepURLs() (map[string]struct{}, error) {
	serviceConfig, err := m.topology.Service(m.serviceURL.AsDereference().String())
	if err != nil {
		return nil, fmt.Errorf("topology.Service: %w", err)
	}

	depURLs := make(map[string]struct{})
	addDep := func(u string) error {
		link, err := m.topology.GetLink(u)
		if err != nil {
			return fmt.Errorf("topology.GetLink('%s'): %w", u, err)
		}
		mushroomURL, err := mushroom.New(link)
		if err != nil {
			return fmt.Errorf("mushroom.New(%q): %w", link, err)
		}
		depURLs[mushroomURL.AsDereference().String()] = struct{}{}
		return nil
	}

	for _, hdep := range serviceConfig.HandlerDeps {
		for _, u := range hdep.Proxies {
			if err := addDep(u); err != nil {
				return nil, err
			}
		}
		for _, u := range hdep.Extensions {
			if err := addDep(u); err != nil {
				return nil, err
			}
		}
	}
	for _, variant := range serviceConfig.Handlers {
		h, ok := variant.AsIndependentHandler()
		if !ok {
			continue
		}
		for _, cdep := range h.CommandDeps {
			for _, u := range cdep.Proxies {
				if err := addDep(u); err != nil {
					return nil, err
				}
			}
			for _, u := range cdep.Extensions {
				if err := addDep(u); err != nil {
					return nil, err
				}
			}
		}
	}

	return depURLs, nil
}

// Handshake waits for deps to become running and exchanges HMAC secrets.
func (m *Manager) Handshake() error {
	depURLs, err := m.getDepURLs()
	if err != nil {
		return fmt.Errorf("getDepURLs: %w", err)
	}
	if len(depURLs) == 0 {
		return nil
	}

	const attempts = 10

	var wg sync.WaitGroup
	errCh := make(chan error, len(depURLs))
	for url := range depURLs {
		wg.Add(1)
		go func(depURL string) {
			defer wg.Done()
			m.testDirectSecureManagerProbe(depURL, "---------SEC before topology reload")
			running, runErr := m.IsServiceRunning(depURL, attempts)
			fmt.Println("m.IsServiceRunning for ", depURL, " running: ", running, ", error: ", runErr)
			m.testDirectSecureManagerProbe(depURL, "---------SEC after topology reload")
			if runErr != nil {
				if errors.Is(runErr, message.ErrAccessDenied) {
					if err := m.handshakeOutbound(depURL); err != nil {
						errCh <- fmt.Errorf("handshakeOutbound(%q): %w", depURL, err)
						return
					}
					running, runErr = m.IsServiceRunning(depURL, attempts)
					m.testDirectSecureManagerProbe(depURL, "---------SEC after handshake")
					fmt.Println("m.IsServiceRunning handshaked ", depURL, " running: ", running, ", error: ", runErr)

				}
				if runErr != nil {
					fmt.Printf("waitServicesRunning: probe error: %v\n", runErr)
					errCh <- fmt.Errorf("IsServiceRunning(%q, attempts: %d): %w", depURL, attempts, runErr)
					return
				}
			}
			if running {
				fmt.Printf("waitServicesRunning: running: %s\n", depURL)
				return
			}
			errCh <- fmt.Errorf("service %q did not become running after %d attempts", depURL, attempts)
		}(url)
	}
	wg.Wait()
	close(errCh)

	for e := range errCh {
		return e
	}
	return nil
}

func (m *Manager) testDirectSecureManagerProbe(depURL string, prefix string) error {
	depService, err := m.topology.Service(depURL)
	if err != nil {
		return fmt.Errorf("topology.Service(%q): %w", depURL, err)
	}

	managerHandler, err := depService.HandlerByCategory(config.ServiceManagerCategory)
	if err != nil {
		return fmt.Errorf("dep %q manager handler: %w", depService.Name, err)
	}
	ind, ok := managerHandler.AsIndependentHandler()
	if !ok {
		return fmt.Errorf("dep %q manager handler is invalid", depService.Name)
	}

	socket, err := protocolClient.New(ind.Endpoint.Id, ind.Endpoint.Port, protocolClient.SyncReplierType)
	if err != nil {
		return fmt.Errorf("client.New: %w", err)
	}
	defer socket.Close()

	node := &topology.Client{Socket: socket}
	if depService.Parameters != nil {
		if pubKey, ok := depService.Parameters[ManagerPublicKeyParam].(string); ok && pubKey != "" {
			node.Socket.Allow(pubKey)
		}
	}
	node.Socket.Secure(m.secretKey)

	node.Attempt(1)
	node.Timeout(100 * time.Millisecond)

	reply, err := node.Request(&message.Request{
		Command:    IsServiceRunning,
		Parameters: datatype.New().Set("service", depService.Name),
	})
	fmt.Println(prefix, "Testing direct manager probing for ", depURL, ": ", reply, ", erro: ", err)
	if err != nil {
		return err
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
	if err := m.Interface.Route(Handshake, m.onHandshake); err != nil {
		return fmt.Errorf(`handler.Route("%s"): %w`, Handshake, err)
	}

	if err := m.setHandlerControls(); err != nil {
		return fmt.Errorf("setHandlerControls: %w", err)
	}

	self, err := m.selfService()
	if err != nil {
		return fmt.Errorf("selfService: %w", err)
	}
	m.selfServiceName = self.Name

	handlerLink := m.serviceURL.New(config.ServiceManagerCategory)
	m.Interface.SetMushroomURL(handlerLink.String())

	m.Interface.Secure(m.secretKey)

	if err := m.Interface.Start(); err != nil {
		return fmt.Errorf("handler.Start: %w", err)
	}

	m.started = true
	m.running = true
	m.startBackgroundHandshake()

	return nil
}
