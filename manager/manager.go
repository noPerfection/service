// Package manager is the manager of the service.
package manager

import (
	"fmt"
	"strings"
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
	"github.com/noPerfection/service/zap"
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
	Handshake                 = "handshake"
	GetServiceStatus          = "service-status"
	SecureOutbounds           = "secure-outbounds"
	SecureInbounds            = "secure-inbounds"
	SecureOutboundConnections = "secure-outbound-cons"
	SecureInboundConnections  = "secure-inbound-cons"
	SecureOutboundEdges       = "secure-outbound-edges"
	SecureInboundEdges        = "secure-inbound-edges"

	// SecureEdges secures topology edges after Handshake using the handshake HMAC secret.
	SecureEdges = "secure-edges"

	SecureEdgesProgressParam         = "progress"
	SecureEdgesProgressManagerInDeps = "manager-in-deps"
	SecureEdgesProgressDepsInDeps    = "deps-in-deps"

	// ManagerPublicKeyParam is the service Parameters key under which the manager's
	// CURVE public key is stored by allowServiceManager.
	ManagerPublicKeyParam = "public-key"
)

// DefaultIndependentManagerEndpoint returns the default endpoint for an independent service's manager.
func DefaultIndependentManagerEndpoint() message.Endpoint {
	return message.NewEndpoint(config.ServiceManagerCategory, 0)
}

// DefaultExtensionManagerEndpoint returns the default endpoint for a service's extension manager.
func DefaultExtensionManagerEndpoint(serviceName string) message.Endpoint {
	return message.NewEndpoint(serviceName+"_ext_"+config.ServiceManagerCategory, 0)
}

// ManagerEndpointForService returns the manager endpoint for a service.
// When the service record has a manager handler, that endpoint is used;
// otherwise the default endpoint for the service type is returned.
func ManagerEndpointForService(service config.Service) (message.Endpoint, error) {
	managerHandler, err := service.HandlerByCategory(config.ServiceManagerCategory)
	if err == nil {
		handler, ok := managerHandler.AsIndependentHandler()
		if !ok {
			return message.Endpoint{}, fmt.Errorf("service %q manager handler is not independent", service.Name)
		}
		return handler.Endpoint, nil
	}

	switch service.Type {
	case config.ProxyType:
		return DefaultProxyManagerEndpoint(service.Name), nil
	case config.ExtensionType:
		return DefaultExtensionManagerEndpoint(service.Name), nil
	case config.IndependentType:
		return DefaultIndependentManagerEndpoint(), nil
	default:
		return message.Endpoint{}, err
	}
}

// DefaultManagerHandlerForService returns the default manager handler config for a service.
// When the service record has a manager handler, that config is returned;
// otherwise the default config for the service type is returned.
func DefaultManagerHandlerForService(service config.Service) (config.Handler, error) {
	endpoint, err := ManagerEndpointForService(service)
	if err != nil {
		return nil, err
	}
	return config.IndependentHandler{
		Type:     config.SyncReplierType,
		Category: config.ServiceManagerCategory,
		Endpoint: endpoint,
	}, nil
}

var _ topology.NodeInterface = (*Manager)(nil)

// The Manager keeps all necessary parameters of the service.
// Manage this service from other parts.
type Manager struct {
	protocolHandler.Interface
	*NodeHandshake
	// mushroomURL of this service in the topology mycelium
	serviceURL mushroom.TopologyURL
	// handler category -> handler control
	handlerControls map[string]*client.Control
	topology        *topology.Client
	blocker         **sync.WaitGroup
	started         bool
	running         bool
	curveSecretKey  string
	pubKey          string
	mu              sync.Mutex
	// manager url (dep service derefence) -> HMAC secret
	outboundHmacSecrets map[string]string
	selfServiceName     string
	logger              *log.Logger
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

	handler := protocolHandler.NewReplier()

	h := &Manager{
		Interface:           handler,
		handlerControls:     make(map[string]*client.Control),
		topology:            topology,
		serviceURL:          serviceURL,
		curveSecretKey:      sec,
		pubKey:              pub,
		outboundHmacSecrets: make(map[string]string),
	}
	nodeHandshake, err := NewHandshake(serviceURL.New(config.ServiceManagerCategory), topology, h)
	if err != nil {
		return nil, fmt.Errorf("NewHandshake: %w", err)
	}
	h.NodeHandshake = nodeHandshake

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
		SecureEdges,
		GetServiceStatus,
		SecureOutbounds,
		SecureInbounds,
		SecureOutboundConnections,
		SecureInboundConnections,
		SecureOutboundEdges,
		SecureInboundEdges,
	}
}

func (m *Manager) getCurveSecret() string {
	return m.curveSecretKey
}

func (m *Manager) isOutboundHmacExist(managerURL string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.outboundHmacSecrets[managerURL]
	return ok
}

func (m *Manager) setOutboundHmacSecret(depServiceURL string, secret string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.outboundHmacSecrets[depServiceURL] = secret
}

func (m *Manager) requireHandlerWhitelist(handlerCategory string, cmd string, secret string, controlTimeout time.Duration) error {
	control, ok := m.handlerControls[handlerCategory]
	if !ok {
		return fmt.Errorf("handler control of %s category is not found", handlerCategory)
	}

	control.Timeout(controlTimeout)
	control.Attempt(2)
	if err := control.RequireWhitelist(cmd, secret); err != nil {
		return fmt.Errorf("control.RequireWhitelist(%q): %w", cmd, err)
	}
	return nil
}

func (m *Manager) requireHandlerSecureOutbound(handlerCategory string, controlTimeout time.Duration) (string, error) {
	control, ok := m.handlerControls[handlerCategory]
	if !ok {
		return "", fmt.Errorf("handler control of %s category is not found", handlerCategory)
	}
	control.Timeout(controlTimeout)
	control.Attempt(2)
	return control.SecureOutbound()
}

func (m *Manager) requireHandlerSecure(handlerCategory string, controlTimeout time.Duration) (string, error) {
	control, ok := m.handlerControls[handlerCategory]
	if !ok {
		return "", fmt.Errorf("handler control of %s category is not found", handlerCategory)
	}
	control.Timeout(controlTimeout)
	control.Attempt(2)
	return control.RequireSecure()
}

func (m *Manager) registerHandlerOutbounds(handlerCategory string, endpoint message.Endpoint, publicKey string, cmd string, secret string, outboundURL string, localCmd string, controlTimeout time.Duration) error {
	control, ok := m.handlerControls[handlerCategory]
	if !ok {
		return fmt.Errorf("handler control of %s category is not found", handlerCategory)
	}
	control.Timeout(controlTimeout)
	control.Attempt(2)
	return control.RegisterOutbounds(endpoint, publicKey, map[string]string{cmd: secret}, outboundURL, localCmd)
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
	m.NodeHandshake.SetLogger(logger)
	return nil
}

func (m *Manager) SetSharedBlocker(blocker **sync.WaitGroup) {
	m.blocker = blocker
}

// NpacPushAnyContext pushes message.Any for functionPath onto npac's handler context stack.
func (m *Manager) NpacPushAnyContext(functionPath any) error {
	ac, ok := protocolHandler.AsAutocontextHandler(m.Interface)
	if !ok {
		return fmt.Errorf("manager handler %T has no autocontext", m.Interface)
	}
	return ac.NpacPushAnyContext(functionPath)
}

// NpacPopAnyContext pops the message.Any route for functionPath from npac's context stack.
func (m *Manager) NpacPopAnyContext(functionPath any) error {
	ac, ok := protocolHandler.AsAutocontextHandler(m.Interface)
	if !ok {
		return fmt.Errorf("manager handler %T has no autocontext", m.Interface)
	}
	return ac.NpacPopAnyContext(functionPath)
}

func (m *Manager) getOutboundHmacSecret(depServiceURL string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.outboundHmacSecrets[depServiceURL]
}

func (m *Manager) getHmacSecret(serviceURL string) string {
	key, err := asTopologyURL(serviceURL, m.topology)
	if err != nil {
		return ""
	}
	return m.getOutboundHmacSecret(key.As(mushroom.SERVICE).AsDereference().String())
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
	return false, fmt.Errorf("Not implemented isServiceRunning")
	// return probeServiceRunning(m.topology, serviceURL, m.curveSecretKey, m.getHmacSecret(serviceURL), attempts...)
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
		service, err := m.topology.Service(serviceURL)
		if err != nil {
			return err
		}
		if err := stopRemoteService(m.topology, serviceURL, m.curveSecretKey, m.getHmacSecret(serviceURL)); err != nil {
			return fmt.Errorf("stopRemoteService(%q): %w", serviceURL, err)
		}
		if !service.IsInproc() {
			return m.topology.StopService(serviceURL)
		}
		return nil
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
	m.handlerControls = make(map[string]*client.Control)

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

func (m *Manager) onSecureEdges(req message.RequestInterface) message.ReplyInterface {
	if m.topology == nil {
		return req.Fail("topology is nil")
	}

	progress, _ := req.RouteParameters().StringValue(SecureEdgesProgressParam)
	if progress == "" {
		return req.Fail("progress is required")
	}

	switch progress {
	case SecureEdgesProgressManagerInDeps:
		outboundsRaw, err := req.RouteParameters().NestedValue("outbounds")
		if err != nil {
			return req.Fail(fmt.Sprintf("req.RouteParameters().NestedValue('outbounds'): %v", err))
		}
		outbounds := make(map[string]string)
		if err := outboundsRaw.Interface(&outbounds); err != nil {
			return req.Fail(fmt.Sprintf("outboundsRaw.Interface: %v", err))
		}
		for depRoute, depURL := range outbounds {
			if depURL == "" {
				return req.Fail(fmt.Sprintf("outbound %q has no dep", depRoute))
			}
			if err := m.allowOutboundManager(depURL); err != nil {
				return req.Fail(fmt.Sprintf("allowOutboundManager(%q from %q): %v", depURL, depRoute, err))
			}
		}
		return req.Ok(datatype.New())
	case SecureEdgesProgressDepsInDeps:
		inboundsRaw, err := req.RouteParameters().NestedValue("inbounds")
		if err != nil {
			return req.Fail(fmt.Sprintf("req.RouteParameters().NestedValue('inbounds'): %v", err))
		}
		inbounds := make(map[string]string)
		if err := inboundsRaw.Interface(&inbounds); err != nil {
			return req.Fail(fmt.Sprintf("inboundsRaw.Interface: %v", err))
		}
		grouped := make(map[string]map[string]string)
		for protectedRouteURL, inboundRouteURL := range inbounds {
			inboundURL, err := asTopologyURL(inboundRouteURL, m.topology)
			if err != nil {
				return req.Fail(fmt.Sprintf("asTopologyURL(%q): %v", inboundRouteURL, err))
			}
			depURL := inboundURL.As(mushroom.SERVICE).AsDereference().String()
			if grouped[depURL] == nil {
				grouped[depURL] = make(map[string]string)
			}
			grouped[depURL][protectedRouteURL] = inboundRouteURL
		}
		for depURL, callerInbounds := range grouped {
			if err := m.handshakeCallerInboundDep(depURL, callerInbounds); err != nil {
				return req.Fail(fmt.Sprintf("handshakeCallerInboundDep(%q): %v", depURL, err))
			}
		}
		return req.Ok(datatype.New())
	default:
		return req.Fail(fmt.Sprintf("unknown secure-edges progress %q", progress))
	}
}

func (m *Manager) allowOutboundManager(outboundRouteURL string) error {
	outboundURL, err := mushroom.Parse(outboundRouteURL)
	if err != nil {
		return fmt.Errorf("mushroom.Parse(%q): %w", outboundRouteURL, err)
	}

	managerLink := outboundURL.As(mushroom.SERVICE).New(config.ServiceManagerCategory)
	pubKey, err := getPublicKeyFromConfig(managerLink.String(), m.topology)
	if err != nil {
		return fmt.Errorf("getPublicKeyFromConfig(%q): %w", managerLink.String(), err)
	}
	if pubKey == "" {
		return fmt.Errorf("manager public key is empty for %q", managerLink.String())
	}

	zap.AuthCurveAdd(m.serviceURL.As(mushroom.HANDLER).String(), pubKey, managerLink.As(mushroom.HANDLER))
	return nil
}

// inbound url within this service is secured. Whitelist is set.
//
// Example (dep-service: entrypoint-proxy, this-service: hello-world)
//
//	Outbounds[entrypoint-proxy.main.hello] = {
//	  RouteURL: entrypoint-proxy.main.hello,
//	  PublicKey: hello-world.main.public-key,
//	  Secret: hello-world.main.secret,
//	}
//
// Outbounds to this service from dep. On Handshake, the service registers inOutbounds.
// onHandshake should return the public key of the entrypoint-proxy.main.hello handler.
// And here it will be recorded as allow via a control.
func (m *Manager) secureInbound(inboundURL mushroom.TopologyURL, secret string, controlTimeout time.Duration) (string, error) {
	if !inboundURL.IsRouteExist() {
		return "", fmt.Errorf("inboundURL.IsRouteExist() is false: %q", inboundURL.String())
	}
	selfServiceURL, err := asTopologyURL(m.serviceURL.String(), m.topology)
	if err != nil {
		return "", fmt.Errorf("asTopologyURL(self): %w", err)
	}
	if !inboundURL.Equal(selfServiceURL, mushroom.SERVICE) {
		return "", fmt.Errorf("inbound route %q is not on this service", inboundURL.String())
	}

	cmd := inboundURL.AdditionalProps["command"]

	handlerCategory := inboundURL.HandlerLink().HandlerCategory()
	if handlerCategory == config.ServiceManagerCategory {
		if err := m.Interface.Whitelist(cmd, secret); err != nil {
			return "", fmt.Errorf("Interface.Whitelist(%q): %w", cmd, err)
		}
		return m.pubKey, nil
	}

	control, ok := m.handlerControls[handlerCategory]
	if !ok {
		return "", fmt.Errorf("handler control of %s category is not found", handlerCategory)
	}
	control.Timeout(controlTimeout)
	control.Attempt(2)
	publicKey, err := control.RequireSecure()
	if err != nil {
		return "", fmt.Errorf("control.RequireSecure(%q): %w", handlerCategory, err)
	}
	zap.AuthDynamicAllow(inboundURL.HandlerLink().String())
	if err := control.RequireWhitelist(cmd, secret); err != nil {
		return "", fmt.Errorf("control.RequireWhitelist(%q): %w", cmd, err)
	}
	return publicKey, nil
}

func (m *Manager) managerControlClient() (*protocolClient.Control, error) {
	service, err := m.selfService()
	if err != nil {
		return nil, fmt.Errorf("selfService: %w", err)
	}
	managerHandler, err := service.HandlerByCategory(config.ServiceManagerCategory)
	if err != nil {
		return nil, fmt.Errorf("manager handler: %w", err)
	}
	ind, ok := managerHandler.AsIndependentHandler()
	if !ok {
		return nil, fmt.Errorf("manager handler is not independent")
	}
	controlEndpoint := protocolHandler.NewInternalControlEndpoint(ind.Endpoint)
	return protocolClient.NewControl(controlEndpoint.Id, controlEndpoint.Port)
}

// registerOutboundContext whitelists in npac the outboundURL for the inboundURL.
// The inboundURL must be in this manager.
func (m *Manager) registerOutboundContext(inboundURL, outboundURL mushroom.TopologyURL, secret, outboundPublicKey string) (string, error) {
	handlerCategory := inboundURL.HandlerLink().HandlerCategory()

	publicKey := ""
	var control *protocolClient.Control
	var err error
	if handlerCategory == config.ServiceManagerCategory {
		publicKey, _ = m.Interface.PublicKey()

		control, err = m.managerControlClient()
		if err != nil {
			return "", fmt.Errorf("m.managerControlClient: %w", err)
		}
		control.Timeout(1 * time.Second)
		control.Attempt(1)
		defer func() { _ = control.Close() }()
	} else {
		control, ok := m.handlerControls[handlerCategory]
		if !ok {
			return "", fmt.Errorf("handler control of %s category is not found", handlerCategory)
		}
		control.Timeout(1 * time.Second)
		control.Attempt(1)
		publicKey, err = control.SecureOutbound()
		if err != nil {
			return "", fmt.Errorf("control.SecureOutbound(%q): %w", handlerCategory, err)
		}
	}

	localCmd := inboundURL.AdditionalProps["command"]
	cmd := outboundURL.AdditionalProps["command"]
	endpoint, err := endpointForRouteURL(outboundURL.String(), m.topology)
	if err != nil {
		return "", fmt.Errorf("endpointForRouteURL(%q): %w", outboundURL, err)
	}

	if err := control.RegisterOutbounds(endpoint, outboundPublicKey, map[string]string{cmd: secret}, outboundURL.String(), localCmd); err != nil {
		return "", fmt.Errorf("control.RegisterOutbounds(%q): %w", outboundURL, err)
	}

	autocontext := protocolClient.NewAutocontext()
	if autocontext == nil {
		return "", fmt.Errorf("failed to create npac autocontext")
	}
	defer func() { _ = autocontext.Close() }()

	remoteHandlerURL := outboundURL.As(mushroom.HANDLER).String()
	if err := autocontext.RegisterOutbound(endpoint, remoteHandlerURL, outboundPublicKey); err != nil {
		if !strings.Contains(err.Error(), "already registered") {
			return "", fmt.Errorf("npac.RegisterOutbound(%q): %w", remoteHandlerURL, err)
		}
	}

	return publicKey, nil
}

func (m *Manager) allowPublicKey(inboundURL string, routeTopologyURL mushroom.TopologyURL, depPublicKey string) error {
	inboundMushroomURL, err := mushroom.Parse(inboundURL)
	if err != nil {
		return fmt.Errorf("mushroom.Parse(%q): %w", inboundURL, err)
	}
	zap.AuthCurveAdd(inboundMushroomURL.As(mushroom.HANDLER).String(), depPublicKey, routeTopologyURL.As(mushroom.HANDLER))
	return nil
}

// allowSelfInDep ensures this manager's CURVE public key is listed in dep's
// parameters.allowed so the dep manager handler can authenticate us.
func (m *Manager) allowSelfInDep(depURL string) error {
	if m.topology == nil {
		return fmt.Errorf("topology is nil")
	}
	if m.pubKey == "" {
		return fmt.Errorf("manager public key is empty")
	}

	depFullURL, err := asTopologyURL(depURL, m.topology)
	if err != nil {
		return fmt.Errorf("asTopologyURL(%q): %w", depURL, err)
	}

	depServiceURL := depFullURL.As(mushroom.SERVICE)
	depService, err := m.topology.Service(depServiceURL.AsDereference().String())
	if err != nil {
		return fmt.Errorf("topology.Service(%q): %w", depServiceURL.AsDereference().String(), err)
	}

	if mushroom.IsAllowedClientPublicKey(&depService, m.pubKey, config.ServiceManagerCategory) {
		return nil
	}

	managerLink := m.serviceURL.New(config.ServiceManagerCategory)
	mushroom.AddAllowedPublicKey(&depService, managerLink, m.serviceURL.ResourcePublicKey())
	if err := m.topology.SetService(depService); err != nil {
		return fmt.Errorf("topology.SetService(%q): %w", depService.Name, err)
	}

	return nil
}

func (m *Manager) handshakeCallerInboundDep(depURL string, callerInbounds map[string]string) error {
	depFullURL, err := asTopologyURL(depURL, m.topology)
	if err != nil {
		return fmt.Errorf("asTopologyURL(%q): %w", depURL, err)
	}

	depServiceURL := depFullURL.As(mushroom.SERVICE)
	secret := message.GenerateSecret()

	m.mu.Lock()
	m.outboundHmacSecrets[depServiceURL.AsDereference().String()] = secret
	m.mu.Unlock()

	managerLink := m.serviceURL.New(config.ServiceManagerCategory)

	depServiceConfig, err := m.topology.Service(depServiceURL.AsDereference().String())
	if err != nil {
		return fmt.Errorf("topology.Service(%q): %w", depServiceURL.AsDereference().String(), err)
	}

	depManagerConfig, err := depServiceConfig.HandlerByCategory(config.ServiceManagerCategory)
	if err != nil {
		return fmt.Errorf("dep %q manager handler: %w", depServiceConfig.Name, err)
	}
	depManagerAsIndependent, ok := depManagerConfig.AsIndependentHandler()
	if !ok {
		return fmt.Errorf("dep %q manager handler is invalid", depServiceConfig.Name)
	}

	socket, err := protocolClient.New(
		depManagerAsIndependent.Endpoint.Id,
		depManagerAsIndependent.Endpoint.Port,
		protocolClient.HandlerType(depManagerAsIndependent.Type),
	)
	if err != nil {
		return fmt.Errorf("client.New: %w", err)
	}
	defer socket.Close()

	node := &topology.Client{Socket: socket}
	if depServiceConfig.Parameters != nil {
		if pubKey, ok := depServiceConfig.Parameters[ManagerPublicKeyParam].(string); ok && pubKey != "" {
			node.Socket.Allow(pubKey)
		}
	}
	node.Socket.Secure(m.curveSecretKey)
	node.Timeout(handshakeRequestTimeout(depServiceConfig))
	node.Attempt(1)

	selfService, err := m.selfService()
	if err != nil {
		return fmt.Errorf("selfService: %w", err)
	}

	inOutbounds := make(map[string]RouteCredential)
	for protectedRouteURL, inboundRouteURL := range callerInbounds {
		protectedURL, err := asTopologyURL(protectedRouteURL, m.topology)
		if err != nil {
			return fmt.Errorf("asTopologyURL(%q): %w", protectedRouteURL, err)
		}
		inboundURL, err := asTopologyURL(inboundRouteURL, m.topology)
		if err != nil {
			return fmt.Errorf("asTopologyURL(%q): %w", inboundRouteURL, err)
		}
		hmacSecret := message.GenerateSecret()
		publicKey, err := m.secureInbound(protectedURL, hmacSecret, handlerControlTimeout(selfService))
		if err != nil {
			return fmt.Errorf("secureInbound(%q): %w", protectedRouteURL, err)
		}
		inOutbounds[inboundURL.String()] = RouteCredential{
			RouteURL:  protectedURL.String(),
			PublicKey: publicKey,
			Secret:    hmacSecret,
		}
	}

	msg := message.Request{
		Command: Handshake,
		Parameters: datatype.New().
			Set("manager-hmac-secret", secret).
			Set("manager-url", managerLink.String()).
			Set("in-inbounds", map[string]RouteCredential{}).
			Set("in-outbounds", inOutbounds),
	}
	signature, err := message.Sign(msg.String(), m.curveSecretKey)
	if err != nil {
		return fmt.Errorf("message.Sign: %w", err)
	}
	msg.Parameters.Set("signature", signature)

	reply, err := node.Request(&msg)
	if err != nil {
		return fmt.Errorf("socket.Request(%q): %w", Handshake, err)
	}
	if !reply.IsOK() {
		return fmt.Errorf("reply.Message: %s", reply.ErrorMessage())
	}

	replyOutboundsKV, err := reply.ReplyParameters().NestedValue("outbounds")
	if err != nil {
		replyOutboundsKV = datatype.New()
	}

	for _, cred := range inOutbounds {
		protectedURL, err := mushroom.Parse(cred.RouteURL)
		if err != nil {
			return fmt.Errorf("mushroom.Parse(%q): %w", cred.RouteURL, err)
		}
		depPublicKey, err := replyOutboundsKV.StringValue(cred.RouteURL)
		if err != nil {
			return fmt.Errorf("reply outbounds public key for %q: %w", cred.RouteURL, err)
		}
		if err := m.allowPublicKey(cred.RouteURL, protectedURL, depPublicKey); err != nil {
			return fmt.Errorf("allowPublicKey(%q): %w", cred.RouteURL, err)
		}
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

	m.handlerControls = make(map[string]*client.Control, len(service.Handlers))
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
		m.handlerControls[handler.Category] = control
	}

	return nil
}

func (m *Manager) whitelistManagerInDeps(depURL string) error {
	depFullURL, err := asTopologyURL(depURL, m.topology)
	if err != nil {
		return fmt.Errorf("asTopologyURL(%q): %w", depURL, err)
	}

	depServiceURL := depFullURL.As(mushroom.SERVICE)

	m.mu.Lock()
	secret := m.outboundHmacSecrets[depServiceURL.AsDereference().String()]
	m.mu.Unlock()
	if secret == "" {
		return fmt.Errorf("dep %q has no self-in-deps handshake secret", depURL)
	}

	depServiceConfig, err := m.topology.Service(depServiceURL.AsDereference().String())
	if err != nil {
		return fmt.Errorf("topology.Service(%q): %w", depServiceURL.AsDereference().String(), err)
	}

	depManagerConfig, err := depServiceConfig.HandlerByCategory(config.ServiceManagerCategory)
	if err != nil {
		return fmt.Errorf("dep %q manager handler: %w", depServiceConfig.Name, err)
	}
	depManagerAsIndependent, ok := depManagerConfig.AsIndependentHandler()
	if !ok {
		return fmt.Errorf("dep %q manager handler is invalid", depServiceConfig.Name)
	}

	socket, err := protocolClient.New(
		depManagerAsIndependent.Endpoint.Id,
		depManagerAsIndependent.Endpoint.Port,
		protocolClient.HandlerType(depManagerAsIndependent.Type),
	)
	if err != nil {
		return fmt.Errorf("client.New: %w", err)
	}
	defer socket.Close()

	node := &topology.Client{Socket: socket}
	depManagerLink := depServiceURL.New(config.ServiceManagerCategory)
	pubKey, err := getPublicKeyFromConfig(depManagerLink.String(), m.topology)
	if err != nil {
		return fmt.Errorf("getPublicKeyFromConfig(%q): %w", depManagerLink.String(), err)
	}
	node.Socket.Allow(pubKey)
	node.Socket.Secure(m.curveSecretKey)
	node.Timeout(handshakeRequestTimeout(depServiceConfig))
	node.Attempt(1)

	depOutbounds := make(map[string]mushroom.TopologyURL)
	// depOutbounds, err := filterTopologyOutbounds(m.topologyOutbounds, depServiceURL, m.serviceURL, true)
	// if err != nil {
	// 	return fmt.Errorf("filterTopologyOutbounds: %w", err)
	// }

	outbounds := make(map[string]string, len(depOutbounds))
	for route, outboundURL := range depOutbounds {
		dep := outboundURL.As(mushroom.SERVICE).AsDereference().String()
		depManagerLink := outboundURL.As(mushroom.SERVICE).New(config.ServiceManagerCategory)
		if _, err := getPublicKeyFromConfig(depManagerLink.String(), m.topology); err != nil {
			continue
		}
		outbounds[route] = dep
	}
	if len(outbounds) == 0 {
		return nil
	}

	if err := node.Socket.Whitelist(message.Any, secret); err != nil {
		return fmt.Errorf("socket.Whitelist(%q): %w", message.Any, err)
	}

	msg := message.Request{
		Command: SecureEdges,
		Parameters: datatype.New().
			Set(SecureEdgesProgressParam, SecureEdgesProgressManagerInDeps).
			Set("outbounds", outbounds),
	}

	reply, err := node.Request(&msg)
	if err != nil {
		return fmt.Errorf("socket.Request(%q): %w", SecureEdges, err)
	}
	if !reply.IsOK() {
		return fmt.Errorf("reply.Message: %s", reply.ErrorMessage())
	}
	return nil
}

func (m *Manager) whitelistDepsInDeps(depURL string) error {
	depFullURL, err := asTopologyURL(depURL, m.topology)
	if err != nil {
		return fmt.Errorf("asTopologyURL(%q): %w", depURL, err)
	}

	depServiceURL := depFullURL.As(mushroom.SERVICE)

	m.mu.Lock()
	secret := m.outboundHmacSecrets[depServiceURL.AsDereference().String()]
	m.mu.Unlock()
	if secret == "" {
		return fmt.Errorf("dep %q has no self-in-deps handshake secret", depURL)
	}

	depServiceConfig, err := m.topology.Service(depServiceURL.AsDereference().String())
	if err != nil {
		return fmt.Errorf("topology.Service(%q): %w", depServiceURL.AsDereference().String(), err)
	}

	depManagerConfig, err := depServiceConfig.HandlerByCategory(config.ServiceManagerCategory)
	if err != nil {
		return fmt.Errorf("dep %q manager handler: %w", depServiceConfig.Name, err)
	}
	depManagerAsIndependent, ok := depManagerConfig.AsIndependentHandler()
	if !ok {
		return fmt.Errorf("dep %q manager handler is invalid", depServiceConfig.Name)
	}

	socket, err := protocolClient.New(
		depManagerAsIndependent.Endpoint.Id,
		depManagerAsIndependent.Endpoint.Port,
		protocolClient.HandlerType(depManagerAsIndependent.Type),
	)
	if err != nil {
		return fmt.Errorf("client.New: %w", err)
	}
	defer socket.Close()

	node := &topology.Client{Socket: socket}
	depManagerLink := depServiceURL.New(config.ServiceManagerCategory)
	pubKey, err := getPublicKeyFromConfig(depManagerLink.String(), m.topology)
	if err != nil {
		return fmt.Errorf("getPublicKeyFromConfig(%q): %w", depManagerLink.String(), err)
	}
	node.Socket.Allow(pubKey)
	node.Socket.Secure(m.curveSecretKey)
	node.Timeout(handshakeRequestTimeout(depServiceConfig))
	node.Attempt(1)

	depInbounds := make(map[string]mushroom.TopologyURL)
	// depInbounds, err := filterTopologyInbounds(m.topologyInbounds, depServiceURL, m.serviceURL, true)
	// if err != nil {
	// 	return fmt.Errorf("filterTopologyInbounds: %w", err)
	// }

	inbounds := make(map[string]string, len(depInbounds))
	for route, inboundURL := range depInbounds {
		inbounds[route] = inboundURL.String()
	}
	if len(inbounds) == 0 {
		return nil
	}

	if err := node.Socket.Whitelist(message.Any, secret); err != nil {
		return fmt.Errorf("socket.Whitelist(%q): %w", message.Any, err)
	}

	msg := message.Request{
		Command: SecureEdges,
		Parameters: datatype.New().
			Set(SecureEdgesProgressParam, SecureEdgesProgressDepsInDeps).
			Set("inbounds", inbounds),
	}

	reply, err := node.Request(&msg)
	if err != nil {
		return fmt.Errorf("socket.Request(%q): %w", SecureEdges, err)
	}
	if !reply.IsOK() {
		return fmt.Errorf("reply.Message: %s", reply.ErrorMessage())
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
	if err := m.Interface.Route(Handshake, m.NodeHandshake.onHandshake); err != nil {
		return fmt.Errorf(`handler.Route("%s"): %w`, Handshake, err)
	}
	if err := m.Interface.Route(GetServiceStatus, m.NodeHandshake.onGetServiceStatus); err != nil {
		return fmt.Errorf(`handler.Route("%s"): %w`, GetServiceStatus, err)
	}
	if err := m.Interface.Route(SecureOutbounds, m.NodeHandshake.onSecureOutbounds); err != nil {
		return fmt.Errorf(`handler.Route("%s"): %w`, SecureOutbounds, err)
	}
	if err := m.Interface.Route(SecureEdges, m.onSecureEdges); err != nil {
		return fmt.Errorf(`handler.Route("%s"): %w`, SecureEdges, err)
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

	m.Interface.Secure(m.curveSecretKey)
	zap.AuthDynamicAllow(handlerLink.String())

	if err := m.Interface.Start(); err != nil {
		return fmt.Errorf("handler.Start: %w", err)
	}
	if err := m.NodeHandshake.start(); err != nil {
		return fmt.Errorf("NodeHandshake.start: %w", err)
	}
	m.NodeHandshake.Print()

	m.started = true
	m.running = true

	return nil
}
