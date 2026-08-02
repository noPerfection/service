// Package manager is the manager of the service.
package manager

import (
	"errors"
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
	Handshake = "handshake"

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
	// mushroomURL of this service in the topology mycelium
	serviceURL mushroom.TopologyURL
	// handler category -> handler control
	handlerControls         map[string]*client.Control
	topology                *topology.Client
	blocker                 **sync.WaitGroup
	started                 bool
	running                 bool
	handshaked              bool
	curveSecretKey          string
	pubKey                  string
	mu                      sync.Mutex
	inboundHmacSecrets      map[string]string // caller manager link -> HMAC secret
	outboundHmacSecrets     map[string]string // dep service dereference URL -> HMAC secret
	selfServiceName         string
	logger                  *log.Logger
	handshakeStop           chan struct{}
	handshakeDone           sync.WaitGroup
	handshakeBackgroundOnce sync.Once
	handshakeInterval       time.Duration
	handshakeMu             sync.Mutex
	topologyInbounds        map[string]map[string][]string // service link -> route link -> inbound route links
	topologyOutbounds       map[string]map[string]string   // dep dereference -> route link -> inbound route links
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
		Interface:       handler,
		handlerControls: make(map[string]*client.Control),
		topology:        topology,
		serviceURL:      serviceURL,
		curveSecretKey:  sec,
		handshaked:      false,
		pubKey:          pub,
		// Inbound Secrets are cached to avoid duplicate handshake
		// manager url -> HMAC secret
		inboundHmacSecrets: make(map[string]string),
		// manager url -> HMAC secret
		outboundHmacSecrets: make(map[string]string),
		handshakeInterval:   defaultHandshakeInterval,
		topologyInbounds:    make(map[string]map[string][]string),
		topologyOutbounds:   make(map[string]map[string]string),
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
		SecureEdges,
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

func (m *Manager) maybeStartBackgroundHandshake() {
	m.handshakeBackgroundOnce.Do(func() {
		m.startBackgroundHandshake()
	})
}

func (m *Manager) startBackgroundHandshake() {
	if m.handshakeInterval < 0 {
		return
	}
	if m.handshakeStop != nil {
		return
	}
	stopCh := make(chan struct{})
	m.handshakeStop = stopCh
	m.handshakeDone.Go(func() {
		timer := time.NewTimer(m.handshakeInterval)
		defer timer.Stop()
		for {
			select {
			case <-stopCh:
				if !timer.Stop() {
					<-timer.C
				}
				return
			case <-timer.C:
				if err := m.Handshake(); err != nil && m.logger != nil {
					m.logger.Warn("background handshake failed", "error", err)
				}
				timer.Reset(m.handshakeInterval)
			}
		}
	})
}

func (m *Manager) stopBackgroundHandshake() {
	if m.handshakeStop == nil {
		return
	}
	close(m.handshakeStop)
	m.handshakeStop = nil
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

func (m *Manager) getHmacSecret(serviceURL string) string {
	key, err := asTopologyURL(serviceURL, m.topology)
	if err != nil {
		return ""
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	return m.outboundHmacSecrets[key.As(mushroom.SERVICE).AsDereference().String()]
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
	return isServiceRunningWithReload(m.topology, serviceURL, m.curveSecretKey, m.getHmacSecret(serviceURL), attempts...)
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

func (m *Manager) onHandshake(req message.RequestInterface) message.ReplyInterface {
	if m.topology == nil {
		return req.Fail("topology is nil")
	}
	managerRawURL, err := req.RouteParameters().StringValue("manager-url")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('manager-url'): %v", err))
	}
	if managerRawURL == "" {
		return req.Fail("manager-url is required")
	}
	managerURL, err := mushroom.Parse(managerRawURL)
	if err != nil {
		return req.Fail(fmt.Sprintf("mushroom.Parse(%q): %v", managerRawURL, err))
	}
	if !managerURL.IsHandlerExist() {
		return req.Fail("manager-url must include a handler category")
	}
	if managerURL.HandlerCategory() != config.ServiceManagerCategory {
		return req.Fail(fmt.Sprintf("manager-url handler category must be %q", config.ServiceManagerCategory))
	}

	signature, err := req.RouteParameters().StringValue("signature")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('signature'): %v", err))
	}
	if signature == "" {
		return req.Fail("signature is required")
	}

	storedPublicKey, err := getPublicKeyFromConfig(managerURL.String(), m.topology)
	if err != nil {
		return req.Fail(err.Error())
	}

	delete(req.RouteParameters(), "signature")
	if err := message.Verify(req.String(), signature, storedPublicKey); err != nil {
		return req.Fail(fmt.Sprintf("message.Verify: %v", err))
	}

	secret, err := req.RouteParameters().StringValue("manager-hmac-secret")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('manager-hmac-secret'): %v", err))
	}
	if secret == "" {
		return req.Fail("manager-hmac-secret is required")
	}

	inboundsRaw, err := req.RouteParameters().NestedValue("in-inbounds")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().NestedValue('in-inbounds'): %v", err))
	}
	inbounds := make(map[string]RouteCredential)
	if err := inboundsRaw.Interface(&inbounds); err != nil {
		return req.Fail(fmt.Sprintf("inboundsRaw.Interface: %v", err))
	}
	outboundsRaw, err := req.RouteParameters().NestedValue("in-outbounds")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().NestedValue('in-outbounds'): %v", err))
	}
	outbounds := make(map[string]RouteCredential)
	if err := outboundsRaw.Interface(&outbounds); err != nil {
		return req.Fail(fmt.Sprintf("outboundsRaw.Interface: %v", err))
	}

	m.mu.Lock()
	if _, ok := m.outboundHmacSecrets[managerURL.String()]; ok {
		m.mu.Unlock()
		return req.Fail("manager-url conflicts with outbound secrets")
	}
	m.inboundHmacSecrets[managerURL.String()] = secret
	m.mu.Unlock()

	// Give access to the manager that handshaked.
	for _, cmd := range managerWhitelistCommands() {
		if err := m.Interface.Whitelist(cmd, secret); err != nil {
			return req.Fail(fmt.Sprintf(`handler.Whitelist("%s"): %v`, cmd, err))
		}
	}

	selfService, err := m.selfService()
	if err != nil {
		return req.Fail(fmt.Sprintf("selfService: %v", err))
	}
	controlTimeout := handlerControlTimeout(selfService)

	replyInbounds := make(map[string]string, len(inbounds))
	for depRoute, cred := range inbounds {
		routeURL, err := mushroom.Parse(depRoute)
		if err != nil {
			return req.Fail(fmt.Sprintf("mushroom.Parse(%q): %v", depRoute, err))
		}
		pubKey, err := m.secureInbound(routeURL, cred.Secret, controlTimeout)
		if err != nil {
			return req.Fail(fmt.Sprintf("secureInbound(%q): %v", depRoute, err))
		}
		if cred.PublicKey != "" {
			if err := m.allowPublicKey(cred.RouteURL, routeURL, cred.PublicKey); err != nil {
				return req.Fail(fmt.Sprintf("allowPublicKey(%q): %v", depRoute, err))
			}
		}
		replyInbounds[depRoute] = pubKey
	}

	replyOutbounds := make(map[string]string, len(outbounds))
	for depRoute, cred := range outbounds {
		if cred.RouteURL == "" {
			continue
		}
		routeURL, err := mushroom.Parse(depRoute)
		if err != nil {
			return req.Fail(fmt.Sprintf("mushroom.Parse(%q): %v", depRoute, err))
		}
		pubKey, err := m.prepareOutboundContext(routeURL)
		if err != nil {
			return req.Fail(fmt.Sprintf("prepareOutboundContext(%q): %v", depRoute, err))
		}
		if err := m.registerOutboundContext(depRoute, cred.RouteURL, cred.Secret, cred.PublicKey); err != nil {
			return req.Fail(fmt.Sprintf("registerOutboundContext(%q): %v", depRoute, err))
		}
		replyOutbounds[cred.RouteURL] = pubKey
	}

	return req.Ok(datatype.New().Set("inbounds", replyInbounds).Set("outbounds", replyOutbounds))
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
	if err := control.RequireWhitelist(cmd, secret); err != nil {
		return "", fmt.Errorf("control.RequireWhitelist(%q): %w", cmd, err)
	}
	return publicKey, nil
}

// prepareOutboundContext returns the control outbound public key for outboundURL.
// The outbound url should be on this service.
//
// When the handler control has no outbound identity yet, an ephemeral CURVE key is generated
// without restarting the handler socket.
//
// Returns public key of the handler control outbound identity.
func (m *Manager) prepareOutboundContext(outboundURL mushroom.TopologyURL) (string, error) {
	if !outboundURL.IsRouteExist() {
		return "", fmt.Errorf("outboundURL.IsRouteExist() is false: %q", outboundURL.String())
	}
	selfServiceURL, err := asTopologyURL(m.serviceURL.String(), m.topology)
	if err != nil {
		return "", fmt.Errorf("asTopologyURL(self): %w", err)
	}
	if !outboundURL.Equal(selfServiceURL, mushroom.SERVICE) {
		return "", fmt.Errorf("outbound route %q is not on this service", outboundURL.String())
	}
	handlerCategory := outboundURL.HandlerLink().HandlerCategory()

	if handlerCategory == config.ServiceManagerCategory {
		publicKey, err := m.Interface.PublicKey()
		if err != nil {
			return "", fmt.Errorf("Interface.PublicKey: %w", err)
		}
		if publicKey == "" {
			return "", fmt.Errorf("manager public key is empty")
		}
		return publicKey, nil
	}

	control, ok := m.handlerControls[handlerCategory]
	if !ok {
		return "", fmt.Errorf("handler control of %s category is not found", handlerCategory)
	}
	selfService, err := m.selfService()
	if err != nil {
		return "", fmt.Errorf("selfService: %w", err)
	}
	control.Timeout(handlerControlTimeout(selfService))
	control.Attempt(2)
	publicKey, err := control.SecureOutbound()
	if err != nil {
		return "", fmt.Errorf("control.SecureOutbound(%q): %w", handlerCategory, err)
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

// registerOutboundContext registers npac + control outbound access from inboundURL to outboundURL.
func (m *Manager) registerOutboundContext(inboundURL, outboundURL, secret, remotePublicKey string) error {
	localURL, err := mushroom.Parse(inboundURL)
	if err != nil {
		return fmt.Errorf("mushroom.Parse(%q): %w", inboundURL, err)
	}
	localCmd := localURL.AdditionalProps["command"]
	if localCmd == "" {
		return fmt.Errorf("route %q has no command", inboundURL)
	}

	remoteURL, err := mushroom.Parse(outboundURL)
	if err != nil {
		return fmt.Errorf("mushroom.Parse(%q): %w", outboundURL, err)
	}
	cmd := remoteURL.AdditionalProps["command"]
	if cmd == "" {
		return fmt.Errorf("route %q has no command", outboundURL)
	}

	endpoint, err := endpointForRouteURL(outboundURL, m.topology)
	if err != nil {
		return fmt.Errorf("endpointForRouteURL(%q): %w", outboundURL, err)
	}

	autocontext := protocolClient.NewAutocontext()
	if autocontext == nil {
		return fmt.Errorf("failed to create npac autocontext")
	}
	defer func() { _ = autocontext.Close() }()

	remoteHandlerURL := remoteURL.As(mushroom.HANDLER).String()
	if err := autocontext.RegisterOutbound(endpoint, remoteHandlerURL, remotePublicKey); err != nil {
		if !strings.Contains(err.Error(), "already registered") {
			return fmt.Errorf("npac.RegisterOutbound(%q): %w", remoteHandlerURL, err)
		}
	}

	handlerCategory := localURL.HandlerLink().HandlerCategory()

	selfService, err := m.selfService()
	if err != nil {
		return fmt.Errorf("selfService: %w", err)
	}

	var control *protocolClient.Control
	if handlerCategory == config.ServiceManagerCategory {
		if m.pubKey == "" {
			return fmt.Errorf("manager public key is empty")
		}

		control, err = m.managerControlClient()
		if err != nil {
			return err
		}
		control.Timeout(managerProbeTimeout(selfService))
		control.Attempt(1)
		defer func() { _ = control.Close() }()
	} else {
		var ok bool
		control, ok = m.handlerControls[handlerCategory]
		if !ok || control == nil {
			return fmt.Errorf("handler control of %s category is not found", handlerCategory)
		}
		control.Timeout(handlerControlTimeout(selfService))
		control.Attempt(1)
	}

	if err := control.RegisterOutbounds(endpoint, remotePublicKey, map[string]string{cmd: secret}, outboundURL, localCmd); err != nil {
		return fmt.Errorf("control.RegisterOutbounds(%q): %w", outboundURL, err)
	}

	return nil
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

// whitelistSelfInDeps is the first of handshaking processes:
//
//	manager-to-manager handshake by whitelisting to all dependencies in topology.
func (m *Manager) whitelistSelfInDeps(depURL string) error {
	depFullURL, err := asTopologyURL(depURL, m.topology)
	if err != nil {
		return fmt.Errorf("asTopologyURL(%q): %w", depURL, err)
	}

	depServiceURL := depFullURL.As(mushroom.SERVICE)

	depOutbounds, err := filterTopologyOutbounds(m.topologyOutbounds, depServiceURL, m.serviceURL, false)
	if err != nil {
		return fmt.Errorf("filterTopologyOutbounds: %w", err)
	}

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
	for route, outboundURL := range depOutbounds {
		hmacSecret := message.GenerateSecret()
		publicKey, err := m.secureInbound(outboundURL, hmacSecret, handlerControlTimeout(selfService))
		if err != nil {
			return fmt.Errorf("secureInbound(%q): %w", route, err)
		}
		inOutbounds[route] = RouteCredential{
			RouteURL:  outboundURL.String(),
			PublicKey: publicKey,
			Secret:    hmacSecret,
		}
	}

	depInbounds, err := filterTopologyInbounds(m.topologyInbounds, depServiceURL, m.serviceURL, false)
	if err != nil {
		return fmt.Errorf("filterTopologyInbounds: %w", err)
	}

	inInbounds := make(map[string]RouteCredential)
	for route, inboundURL := range depInbounds {
		hmacSecret := message.GenerateSecret()
		publicKey, err := m.prepareOutboundContext(inboundURL)
		if err != nil {
			return fmt.Errorf("prepareInboundCredential: %w", err)
		}
		inInbounds[route] = RouteCredential{
			RouteURL:  inboundURL.String(),
			PublicKey: publicKey,
			Secret:    hmacSecret,
		}
	}

	msg := message.Request{
		Command: Handshake,
		Parameters: datatype.New().
			Set("manager-hmac-secret", secret).
			Set("manager-url", managerLink.String()).
			Set("in-inbounds", inInbounds).
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

	replyInboundsKV, err := reply.ReplyParameters().NestedValue("inbounds")
	if err != nil {
		replyInboundsKV = datatype.New()
	}

	for outboundRouteURL, cred := range inInbounds {
		localRoute := cred.RouteURL
		depPublicKey, err := replyInboundsKV.StringValue(outboundRouteURL)
		if err != nil {
			return fmt.Errorf("reply inbounds public key for %q: %w", outboundRouteURL, err)
		}
		if err := m.registerOutboundContext(cred.RouteURL, outboundRouteURL, cred.Secret, depPublicKey); err != nil {
			return fmt.Errorf("registerOutboundContext(%q): %w", localRoute, err)
		}
	}

	replyOutboundsKV, err := reply.ReplyParameters().NestedValue("outbounds")
	if err != nil {
		replyOutboundsKV = datatype.New()
	}

	for route, cred := range inOutbounds {
		routeTopologyURL, ok := depOutbounds[route]
		if !ok {
			return fmt.Errorf("outbound topology for %q is not found", route)
		}
		depPublicKey, err := replyOutboundsKV.StringValue(cred.RouteURL)
		if err != nil {
			return fmt.Errorf("reply outbounds public key for %q: %w", cred.RouteURL, err)
		}
		if err := m.allowPublicKey(cred.RouteURL, routeTopologyURL, depPublicKey); err != nil {
			return fmt.Errorf("allowPublicKey(%q): %w", route, err)
		}
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

// Returns handler dereferences as full paths built from dependencies.
// Dependency URLs may be service or handler links; when no handler category
// is present (e.g. proxy or extension deps), the default category (main) is applied.
func (m *Manager) getDepDereferences() (map[string]struct{}, error) {
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
		mushroomURL, err := mushroom.Parse(link)
		if err != nil {
			return fmt.Errorf("mushroom.Parse(%q): %w", link, err)
		}
		depURLs[mushroomURL.HandlerLink().AsDereference().String()] = struct{}{}
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

func (m *Manager) buildTopologyOutbounds() error {
	depDerefs, err := m.getDepDereferences()
	if err != nil {
		return fmt.Errorf("getDepDereferences: %w", err)
	}

	outbounds, err := buildTopologyOutbounds(m.topologyInbounds, depDerefs, m.serviceURL.AsDereference().String())
	if err != nil {
		return fmt.Errorf("buildWhitelistedOutbounds: %w", err)
	}

	m.topologyOutbounds = outbounds
	//logWhitelistInbounds(m.logger, outbounds, depDerefs)
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

	depOutbounds, err := filterTopologyOutbounds(m.topologyOutbounds, depServiceURL, m.serviceURL, true)
	if err != nil {
		return fmt.Errorf("filterTopologyOutbounds: %w", err)
	}

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

	depInbounds, err := filterTopologyInbounds(m.topologyInbounds, depServiceURL, m.serviceURL, true)
	if err != nil {
		return fmt.Errorf("filterTopologyInbounds: %w", err)
	}

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

// Handshake waits for deps to become running and exchanges HMAC secrets.
func (m *Manager) Handshake() error {
	m.handshakeMu.Lock()
	defer m.handshakeMu.Unlock()

	depURLs, err := m.getDepDereferences()
	if err != nil {
		return fmt.Errorf("getDepURLs: %w", err)
	}
	if len(depURLs) == 0 {
		m.maybeStartBackgroundHandshake()
		return nil
	}

	const attempts = 3

	var wg sync.WaitGroup
	var mu sync.Mutex
	selfInDepsDone := make([]string, 0, len(depURLs))
	errCh := make(chan error, len(depURLs)*2)
	for url := range depURLs {
		wg.Add(1)
		go func(depURL string) {
			defer wg.Done()
			start := time.Now()
			fmt.Printf("> is %s service running, attempts %d, time: %s\n", depURL, attempts, start)
			running, runErr := m.IsServiceRunning(depURL, attempts)
			handshaked := false
			fmt.Printf("< is %s service running? %t, err: %v, time: %s\n", depURL, running, runErr, time.Since(start))
			if runErr != nil {
				if errors.Is(runErr, message.ErrAccessDenied) {
					if err := m.whitelistSelfInDeps(depURL); err != nil {
						errCh <- fmt.Errorf("whitelistSelfInDeps(%q): %w", depURL, err)
						return
					}
					handshaked = true
					running, runErr = m.IsServiceRunning(depURL, 1)
				} else if errors.Is(runErr, message.ErrNoCurveKey) {
					if err := m.allowSelfInDep(depURL); err != nil {
						errCh <- fmt.Errorf("allowSelfInDep(%q): %w", depURL, err)
						return
					}
					running, runErr = m.IsServiceRunning(depURL, 1)
					if runErr != nil {
						if err := m.whitelistSelfInDeps(depURL); err != nil {
							errCh <- fmt.Errorf("whitelistSelfInDeps(%q): %w", depURL, err)
							return
						}
						handshaked = true
						running, runErr = m.IsServiceRunning(depURL, 1)
					}
				}
				if runErr != nil {
					errCh <- fmt.Errorf("IsServiceRunning(%q, attempts: %d): %w", depURL, attempts, runErr)
					return
				}
			}
			if !running {
				errCh <- fmt.Errorf("service %q not running, attempts: %d", depURL, attempts)
				return
			}
			if handshaked {
				mu.Lock()
				selfInDepsDone = append(selfInDepsDone, depURL)
				mu.Unlock()
			}
		}(url)
	}
	wg.Wait()

	managerInDepWhitelisted := make(map[string]bool, len(selfInDepsDone))
	for _, depURL := range selfInDepsDone {
		wg.Add(1)
		go func(depURL string) {
			defer wg.Done()
			if err := m.whitelistManagerInDeps(depURL); err != nil {
				errCh <- fmt.Errorf("whitelistManagerInDeps(%q): %w", depURL, err)
				return
			}
			mu.Lock()
			managerInDepWhitelisted[depURL] = true
			mu.Unlock()
		}(depURL)
	}
	wg.Wait()

	for _, depURL := range selfInDepsDone {
		if !managerInDepWhitelisted[depURL] {
			continue
		}
		wg.Add(1)
		go func(depURL string) {
			defer wg.Done()
			if err := m.whitelistDepsInDeps(depURL); err != nil {
				errCh <- fmt.Errorf("whitelistDepsInDeps(%q): %w", depURL, err)
			}
		}(depURL)
	}
	wg.Wait()
	close(errCh)

	for e := range errCh {
		return e
	}

	m.maybeStartBackgroundHandshake()
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

	m.started = true
	m.running = true
	inbounds, err := getRouteInbounds(m)
	if err != nil {
		return fmt.Errorf("getRouteInbounds: %w", err)
	}
	m.topologyInbounds = inbounds
	if err := m.buildTopologyOutbounds(); err != nil {
		return fmt.Errorf("buildTopologyOutbounds: %w", err)
	}

	return nil
}
