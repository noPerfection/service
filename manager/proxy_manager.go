package manager

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/log"
	protocolClient "github.com/noPerfection/protocol/client"
	protocolHandler "github.com/noPerfection/protocol/handler"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service/handlers"
	"github.com/noPerfection/service/mushroom"
	"github.com/noPerfection/service/zap"
	"github.com/noPerfection/topology"
	"github.com/noPerfection/topology/config"
	topologyConfig "github.com/noPerfection/topology/config"
)

var _ topology.NodeInterface = (*ProxyManager)(nil)

// DefaultProxyManagerEndpoint returns the default endpoint for a service's proxy manager.
func DefaultProxyManagerEndpoint(serviceName string) message.Endpoint {
	return message.NewEndpoint(serviceName+"_proxy_"+topologyConfig.ServiceManagerCategory, 0)
}

// ProxyManager keeps all necessary parameters of the proxy service.
// Manage this proxy service from other parts.
type ProxyManager struct {
	protocolHandler.Interface
	*NodeHandshake
	serviceName string
	topology    *topology.Client
	setup       *protocolClient.PairClient
	setupMu     sync.Mutex
	blocker     **sync.WaitGroup
	running     bool
	logger      *log.Logger
	secretKey   string
	pubKey      string
	mu          sync.Mutex
	// service url -> secret cached
	inboundSecrets map[string]string
	// service url -> secret
	outboundSecrets         map[string]string
	handshakeStop           chan struct{}
	handshakeDone           sync.WaitGroup
	handshakeBackgroundOnce sync.Once
	handshakeInterval       time.Duration
	handshakeMu             sync.Mutex
	inbounds                map[string]map[string][]string
	outbounds               map[string]map[string]string
	whitelistedOutbounds    map[string]struct{}
}

// NewProxyManager creates a manager for a proxy service.
// An optional secretKey may be provided; if given, the public key is derived from it.
// If omitted, a fresh CURVE keypair is generated.
func NewProxyManager(serviceName string, managerEndpoint message.Endpoint, secretKey ...string) (*ProxyManager, error) {
	if serviceName == "" {
		return nil, fmt.Errorf("serviceName is required")
	}

	topologyClient, err := topology.NewClient()
	if err != nil {
		return nil, fmt.Errorf("topology.NewClient: %w", err)
	}

	proxyHandlersClient, err := protocolClient.NewPair(serviceName+handlers.ProxyHandlersCategory, 0)
	if err != nil {
		_ = topologyClient.Close()
		return nil, fmt.Errorf("client.NewPair('%s'): %w", serviceName+handlers.ProxyHandlersCategory, err)
	}

	var pub, sec string
	if len(secretKey) > 0 && secretKey[0] != "" {
		sec = secretKey[0]
		pub, err = message.DerivePublicKey(sec)
		if err != nil {
			_ = topologyClient.Close()
			_ = proxyHandlersClient.Close()
			return nil, fmt.Errorf("message.DerivePublicKey: %w", err)
		}
	} else {
		pub, sec, err = message.GenerateCurveKey()
		if err != nil {
			_ = topologyClient.Close()
			_ = proxyHandlersClient.Close()
			return nil, fmt.Errorf("message.GenerateCurveKey: %w", err)
		}
	}

	handler := protocolHandler.NewSyncReplier()

	fmt.Printf("NewProxyManager: %v\n", managerEndpoint.ClientUrl())

	h := &ProxyManager{
		Interface:            handler,
		NodeHandshake:        nil,
		topology:             topologyClient,
		setup:                proxyHandlersClient,
		serviceName:          serviceName,
		secretKey:            sec,
		pubKey:               pub,
		inboundSecrets:       make(map[string]string),
		outboundSecrets:      make(map[string]string),
		handshakeInterval:    defaultHandshakeInterval,
		inbounds:             make(map[string]map[string][]string),
		outbounds:            make(map[string]map[string]string),
		whitelistedOutbounds: make(map[string]struct{}),
	}

	handler.SetEndpoint(managerEndpoint)

	return h, nil
}

// PublicKey returns the CURVE public key for this proxy manager's handler.
func (m *ProxyManager) PublicKey() string {
	return m.pubKey
}

func proxyManagerWhitelistCommands() []string {
	return []string{
		IsServiceRunning,
		StartService,
		StopService,
		Services,
		SecureEdges,
		handlers.SetProxyHandlerCommand,
		handlers.IsProxyHandlerExistCommand,
		handlers.IsProxyHandlerRunningCommand,
		handlers.StartProxyHandlerCommand,
		handlers.StopProxyHandlerCommand,
		handlers.RemoveProxyHandlerCommand,
	}
}

// RequireWhitelist marks every proxy manager route except handshake as requiring a whitelist entry.
func (m *ProxyManager) RequireWhitelist() {
	for _, cmd := range proxyManagerWhitelistCommands() {
		m.Interface.RequireWhitelist(cmd)
	}
}

func (m *ProxyManager) whitelistSecret(secret string) error {
	for _, cmd := range proxyManagerWhitelistCommands() {
		if err := m.Interface.Whitelist(cmd, secret); err != nil {
			return fmt.Errorf(`handler.Whitelist("%s"): %w`, cmd, err)
		}
	}
	return nil
}

func (m *ProxyManager) SetSharedBlocker(blocker **sync.WaitGroup) {
	m.blocker = blocker
}

// NpacPushAnyContext pushes message.Any for functionPath onto npac's handler context stack.
func (m *ProxyManager) NpacPushAnyContext(functionPath any) error {
	ac, ok := protocolHandler.AsAutocontextHandler(m.Interface)
	if !ok {
		return fmt.Errorf("proxy manager handler %T has no autocontext", m.Interface)
	}
	return ac.NpacPushAnyContext(functionPath)
}

// NpacPopAnyContext pops the message.Any route for functionPath from npac's context stack.
func (m *ProxyManager) NpacPopAnyContext(functionPath any) error {
	ac, ok := protocolHandler.AsAutocontextHandler(m.Interface)
	if !ok {
		return fmt.Errorf("proxy manager handler %T has no autocontext", m.Interface)
	}
	return ac.NpacPopAnyContext(functionPath)
}

// SetLogger sets the optional logger for the proxy manager.
func (m *ProxyManager) SetLogger(logger *log.Logger) error {
	m.logger = logger
	if m.Interface == nil {
		return nil
	}
	if err := m.Interface.SetLogger(logger); err != nil {
		return fmt.Errorf("manager SetLogger: %w", err)
	}
	if m.NodeHandshake != nil {
		m.NodeHandshake.SetLogger(logger)
	}
	return nil
}

func (m *ProxyManager) maybeStartBackgroundHandshake() {
	m.handshakeBackgroundOnce.Do(func() {
		m.startBackgroundHandshake()
	})
}

func (m *ProxyManager) startBackgroundHandshake() {
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

func (m *ProxyManager) stopBackgroundHandshake() {
	if m.handshakeStop == nil {
		return
	}
	close(m.handshakeStop)
	m.handshakeStop = nil
}

// For now, let's make it not starting. It just returns its own name.
// Later it will just keep almost identical to Start() data.
func (m *ProxyManager) StartService(serviceName string) (string, error) {
	if serviceName == "" || serviceName == m.serviceName {
		if err := m.ensureTopologyClient(); err != nil {
			return "", err
		}
		if err := m.ensureProxyHandlersClient(); err != nil {
			return "", err
		}
		if err := m.setProxyHandlers(); err != nil {
			return "", fmt.Errorf("setProxyHandlers: %w", err)
		}
		if err := m.proxyHandlersRequest(handlers.StartProxyHandlersCommand); err != nil {
			return "", err
		}

		m.running = true
		return strconv.Itoa(os.Getpid()), nil
	}
	if id, ok, err := m.startInprocServiceIfNeeded(serviceName); ok || err != nil {
		return id, err
	}
	return "", fmt.Errorf("service name is not empty and not equal to the service name")
}

func (m *ProxyManager) getHmacSecret(serviceURL string) string {
	if err := m.ensureTopologyClient(); err != nil {
		return ""
	}
	key, err := asTopologyURL(serviceURL, m.topology)
	if err != nil {
		return ""
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	return m.outboundSecrets[key.As(mushroom.SERVICE).AsDereference().String()]
}

// For now, lets just return manager.running.
func (m *ProxyManager) IsServiceRunning(serviceURL string, attempts ...int) (bool, error) {
	fmt.Printf(">> IsServiceRunning(%q): running? %v\n", serviceURL, m.running)
	if serviceURL == m.serviceName {
		return m.running, nil
	}

	if err := m.ensureTopologyClient(); err != nil {
		return false, err
	}

	serviceConfig, err := m.topology.Service(serviceURL)
	if err != nil {
		return false, fmt.Errorf("topology.Service(%q): %w", serviceURL, err)
	}
	if serviceConfig.Name == m.serviceName {
		fmt.Printf(">> IsServiceRunning(%q): %v\n", serviceURL, m.running)
		return m.running, nil
	}
	fmt.Printf(">> IsServiceRunning(%q): %v\n", serviceURL, false)
	return false, fmt.Errorf("Not implemented isServiceRunning for " + serviceURL)
	// return isServiceRunningWithReload(m.topology, serviceName, m.secretKey, m.getHmacSecret(serviceName), attempts...)
}

func (m *ProxyManager) StopService(serviceName string) error {
	if serviceName == "" || serviceName == m.serviceName {
		m.mu.Lock()
		if !m.running {
			m.mu.Unlock()
			return nil
		}
		m.running = false
		m.mu.Unlock()

		var stopErr error
		defer func() {
			if m.blocker != nil && *m.blocker != nil {
				(*m.blocker).Done()
			}
		}()

		m.stopBackgroundHandshake()

		if m.setup != nil {
			if err := m.proxyHandlersRequest(handlers.StopProxyHandlersCommand); err != nil {
				stopErr = err
			} else if err := m.setup.Close(); err != nil {
				stopErr = fmt.Errorf("proxyHandlersClient.Close: %w", err)
			}
			m.setup = nil
		}
		if m.topology != nil {
			if err := m.topology.Close(); err != nil && stopErr == nil {
				stopErr = fmt.Errorf("topologyClient.Close: %w", err)
			}
			m.topology = nil
		}

		if err := handlers.CloseViaControl(m.Interface); err != nil && stopErr == nil {
			stopErr = fmt.Errorf("manager handler close: %w", err)
		}

		return stopErr
	}
	if err := m.ensureTopologyClient(); err != nil {
		return err
	}
	service, err := m.topology.Service(serviceName)
	if err != nil {
		return err
	}
	if err := stopRemoteService(m.topology, serviceName, m.secretKey, m.getHmacSecret(serviceName)); err != nil {
		return fmt.Errorf("stopRemoteService(%q): %w", serviceName, err)
	}
	if !service.IsInproc() {
		return m.topology.StopService(serviceName)
	}
	return nil
}

func (m *ProxyManager) inprocServiceRecord(serviceName string) (topologyConfig.Service, bool, error) {
	if err := m.ensureTopologyClient(); err != nil {
		return topologyConfig.Service{}, false, err
	}
	record, err := m.topology.Service(serviceName)
	if err != nil {
		return topologyConfig.Service{}, false, nil
	}
	if !record.IsInproc() {
		return topologyConfig.Service{}, false, nil
	}
	return record, true, nil
}

func (m *ProxyManager) startInprocServiceIfNeeded(serviceName string) (string, bool, error) {
	record, ok, err := m.inprocServiceRecord(serviceName)
	if err != nil || !ok {
		return "", false, err
	}
	endpoint, handlerType, err := inprocTopologyExtensionEndpoint(m.topology)
	if err != nil {
		return "", true, err
	}
	id, err := startInprocService(endpoint, handlerType, record.Name)
	return id, true, err
}

// Close closes the manager, and service as well.
func (m *ProxyManager) Close() error {
	if m == nil {
		return fmt.Errorf("manager is nil")
	}

	if err := m.StopService(m.serviceName); err != nil {
		return err
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

	if err := m.StopService(serviceName); err != nil {
		return req.Fail(fmt.Sprintf("manager.StopService('%s'): %v", serviceName, err))
	}

	return req.Ok(datatype.New())
}

func (m *ProxyManager) onServices(req message.RequestInterface) message.ReplyInterface {
	if m.topology == nil {
		return req.Fail("topologyClient is nil")
	}

	services, err := m.topology.Services()
	if err != nil {
		return req.Fail(fmt.Sprintf("topologyClient.Services: %v", err))
	}

	return req.Ok(datatype.New().Set("services", services))
}

func (m *ProxyManager) proxySelfService() (topologyConfig.Service, error) {
	if err := m.ensureTopologyClient(); err != nil {
		return topologyConfig.Service{}, err
	}
	service, err := m.topology.Service(m.serviceName)
	if err != nil {
		return topologyConfig.Service{}, fmt.Errorf("topology.Service(%q): %w", m.serviceName, err)
	}
	return service, nil
}

func (m *ProxyManager) proxyServiceURL() (mushroom.TopologyURL, error) {
	if err := m.ensureTopologyClient(); err != nil {
		return mushroom.TopologyURL{}, err
	}
	serviceLink, err := m.topology.GetLink(m.serviceName)
	if err != nil {
		return mushroom.TopologyURL{}, fmt.Errorf("topology.GetLink(%q): %w", m.serviceName, err)
	}
	return mushroom.Parse(serviceLink)
}

func routeCredentialFromRaw(raw interface{}) (RouteCredential, bool) {
	var cred RouteCredential
	credKV, ok := raw.(datatype.KeyValue)
	if !ok {
		if rawMap, ok := raw.(map[string]interface{}); ok {
			credKV = rawMap
		} else {
			return RouteCredential{}, false
		}
	}
	if err := credKV.Interface(&cred); err != nil {
		return RouteCredential{}, false
	}
	return cred, true
}

func (m *ProxyManager) secureInbound(inboundURL mushroom.TopologyURL, secret string, controlTimeout time.Duration) (string, error) {
	serviceURL, err := m.proxyServiceURL()
	if err != nil {
		return "", err
	}
	if inboundURL.As(mushroom.SERVICE).AsDereference().String() != serviceURL.AsDereference().String() {
		return "", fmt.Errorf("inbound route %q is not on this service", inboundURL.String())
	}

	cmd := inboundURL.AdditionalProps["command"]
	if cmd == "" {
		return "", fmt.Errorf("route %q has no command", inboundURL.String())
	}

	handlerCategory := inboundURL.HandlerLink().HandlerCategory()
	if handlerCategory == topologyConfig.ServiceManagerCategory {
		if !m.Interface.IsSecure() {
			m.Interface.Secure(m.secretKey)
			zap.AuthDynamicAllow(m.Interface.MushroomURL())
		}
		if err := m.Interface.Whitelist(cmd, secret); err != nil {
			return "", fmt.Errorf("Interface.Whitelist(%q): %w", cmd, err)
		}
		if m.pubKey == "" {
			return "", fmt.Errorf("manager public key is empty")
		}
		return m.pubKey, nil
	}

	publicKey, err := m.proxySetupRequireSecure(handlerCategory, controlTimeout)
	if err != nil {
		return "", fmt.Errorf("proxySetupRequireSecure(%q): %w", handlerCategory, err)
	}
	if err := m.proxySetupRequireInboundWhitelist(handlerCategory, cmd, secret); err != nil {
		return "", fmt.Errorf("proxySetupRequireInboundWhitelist(%q): %w", cmd, err)
	}
	return publicKey, nil
}

func (m *ProxyManager) allowPublicKey(inboundURL string, routeTopologyURL mushroom.TopologyURL, depPublicKey string) error {
	if depPublicKey == "" {
		return nil
	}
	inboundMushroomURL, err := mushroom.Parse(inboundURL)
	if err != nil {
		return fmt.Errorf("mushroom.Parse(%q): %w", inboundURL, err)
	}
	zap.AuthCurveAdd(inboundMushroomURL.As(mushroom.HANDLER).String(), depPublicKey, routeTopologyURL.As(mushroom.HANDLER))
	return nil
}

// allowSelfInDep ensures this manager's CURVE public key is listed in dep's
// parameters.allowed so the dep manager handler can authenticate us.
func (m *ProxyManager) allowSelfInDep(depURL string) error {
	if err := m.ensureTopologyClient(); err != nil {
		return err
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

	if mushroom.IsAllowedClientPublicKey(&depService, m.pubKey, topologyConfig.ServiceManagerCategory) {
		return nil
	}

	serviceURL, err := m.proxyServiceURL()
	if err != nil {
		return fmt.Errorf("proxyServiceURL: %w", err)
	}

	managerLink := serviceURL.New(topologyConfig.ServiceManagerCategory)
	mushroom.AddAllowedPublicKey(&depService, managerLink, serviceURL.ResourcePublicKey())
	if err := m.topology.SetService(depService); err != nil {
		return fmt.Errorf("topology.SetService(%q): %w", depService.Name, err)
	}

	return nil
}

func (m *ProxyManager) prepareOutboundContext(outboundURL mushroom.TopologyURL) (string, error) {
	serviceURL, err := m.proxyServiceURL()
	if err != nil {
		return "", err
	}
	if outboundURL.As(mushroom.SERVICE).AsDereference().String() != serviceURL.AsDereference().String() {
		return "", fmt.Errorf("outbound route %q is not on this service", outboundURL.String())
	}

	handlerCategory := outboundURL.HandlerLink().HandlerCategory()
	if handlerCategory == topologyConfig.ServiceManagerCategory {
		if !m.Interface.IsSecure() {
			m.Interface.Secure(m.secretKey)
			zap.AuthDynamicAllow(m.Interface.MushroomURL())
		}
		if m.pubKey == "" {
			return "", fmt.Errorf("manager public key is empty")
		}
		return m.pubKey, nil
	}

	selfService, err := m.proxySelfService()
	if err != nil {
		return "", fmt.Errorf("proxySelfService: %w", err)
	}
	publicKey, err := m.proxySetupSecureOutbound(handlerCategory, handlerControlTimeout(selfService))
	if err != nil {
		return "", fmt.Errorf("proxySetupSecureOutbound(%q): %w", handlerCategory, err)
	}
	return publicKey, nil
}

func (m *ProxyManager) proxyEndpointForRouteURL(routeURL string) (message.Endpoint, error) {
	routeMushroomURL, err := mushroom.Parse(routeURL)
	if err != nil {
		return message.Endpoint{}, fmt.Errorf("mushroom.Parse(%q): %w", routeURL, err)
	}

	service, err := m.topology.Service(routeMushroomURL.As(mushroom.SERVICE).AsDereference().String())
	if err != nil {
		return message.Endpoint{}, fmt.Errorf("topology.Service: %w", err)
	}

	handler, err := service.HandlerByCategory(routeMushroomURL.HandlerLink().HandlerCategory())
	if err != nil {
		return message.Endpoint{}, fmt.Errorf("HandlerByCategory(%q): %w", routeMushroomURL.HandlerLink().HandlerCategory(), err)
	}
	ind, ok := handler.AsIndependentHandler()
	if !ok {
		return message.Endpoint{}, fmt.Errorf("route %q is not an independent handler", routeURL)
	}

	return ind.Endpoint, nil
}

func (m *ProxyManager) proxySetupRequireSecure(category string, timeout time.Duration) (string, error) {
	params := datatype.New().Set("category", category)
	if timeout > 0 {
		params.Set("timeout", uint64(timeout))
	}

	reply, err := m.proxySetupRoundTrip(&message.Request{
		Command:    handlers.RequireSecureHandlerCommand,
		Parameters: params,
	})
	if err != nil {
		return "", err
	}
	if !reply.IsOK() {
		return "", fmt.Errorf("proxySetup.Receive(%q): %s", handlers.RequireSecureHandlerCommand, reply.ErrorMessage())
	}

	pubKey, err := reply.ReplyParameters().StringValue("public-key")
	if err != nil {
		return "", fmt.Errorf("reply.ReplyParameters().StringValue('public-key'): %w", err)
	}
	return pubKey, nil
}

func (m *ProxyManager) proxySetupSecureOutbound(category string, timeout time.Duration) (string, error) {
	params := datatype.New().Set("category", category)
	if timeout > 0 {
		params.Set("timeout", uint64(timeout))
	}

	reply, err := m.proxySetupRoundTrip(&message.Request{
		Command:    handlers.SecureOutboundHandlerCommand,
		Parameters: params,
	})
	if err != nil {
		return "", err
	}
	if !reply.IsOK() {
		return "", fmt.Errorf("proxySetup.Receive(%q): %s", handlers.SecureOutboundHandlerCommand, reply.ErrorMessage())
	}

	pubKey, err := reply.ReplyParameters().StringValue("public-key")
	if err != nil {
		return "", fmt.Errorf("reply.ReplyParameters().StringValue('public-key'): %w", err)
	}
	return pubKey, nil
}

func (m *ProxyManager) proxySetupRequireInboundWhitelist(category, cmd, secret string) error {
	params := datatype.New().
		Set("category", category).
		Set("command", cmd)
	if secret != "" {
		params.Set("secret", secret)
	}

	reply, err := m.proxySetupRoundTrip(&message.Request{
		Command:    handlers.RequireInboundWhitelistCommand,
		Parameters: params,
	})
	if err != nil {
		return err
	}
	if !reply.IsOK() {
		return fmt.Errorf("proxySetup.Receive(%q): %s", handlers.RequireInboundWhitelistCommand, reply.ErrorMessage())
	}
	return nil
}

func (m *ProxyManager) proxySetupRegisterOutbounds(category string, endpoint message.Endpoint, publicKey string, commands map[string]string, outboundURL, localCmd string) error {
	endpointKV, err := datatype.NewFromInterface(endpoint)
	if err != nil {
		return fmt.Errorf("datatype.NewFromInterface(endpoint): %w", err)
	}
	commandsKV, err := datatype.NewFromInterface(commands)
	if err != nil {
		return fmt.Errorf("datatype.NewFromInterface(commands): %w", err)
	}

	params := datatype.New().
		Set("category", category).
		Set("endpoint", endpointKV).
		Set("commands", commandsKV)
	if publicKey != "" {
		params.Set("public-key", publicKey)
	}
	if outboundURL != "" {
		params.Set("outbound-url", outboundURL)
	}
	if localCmd != "" {
		params.Set("local-command", localCmd)
	}

	reply, err := m.proxySetupRoundTrip(&message.Request{
		Command:    handlers.RegisterHandlerOutboundsCommand,
		Parameters: params,
	})
	if err != nil {
		return err
	}
	if !reply.IsOK() {
		return fmt.Errorf("proxySetup.Receive(%q): %s", handlers.RegisterHandlerOutboundsCommand, reply.ErrorMessage())
	}
	return nil
}

func (m *ProxyManager) proxyManagerControlClient() (*protocolClient.Control, error) {
	endpoint := m.Interface.Endpoint()
	if endpoint == (message.Endpoint{}) {
		return nil, fmt.Errorf("proxy manager endpoint is not set")
	}
	controlEndpoint := protocolHandler.NewInternalControlEndpoint(endpoint)
	return protocolClient.NewControl(controlEndpoint.Id, controlEndpoint.Port)
}

func (m *ProxyManager) registerOutboundContext(inboundURL, outboundURL, secret, remotePublicKey string) error {
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

	endpoint, err := m.proxyEndpointForRouteURL(outboundURL)
	if err != nil {
		return err
	}

	if m.topology == nil {
		return fmt.Errorf("topology is nil")
	}
	remoteService, err := m.topology.Service(remoteURL.As(mushroom.SERVICE).AsDereference().String())
	if err != nil {
		return fmt.Errorf("topology.Service(%q): %w", remoteURL.As(mushroom.SERVICE).AsDereference().String(), err)
	}

	handlerCategory := localURL.HandlerLink().HandlerCategory()
	remoteHandlerCategory := remoteURL.HandlerLink().HandlerCategory()
	commands := map[string]string{cmd: secret}

	switch remoteService.Type {
	case topologyConfig.ProxyType, topologyConfig.IndependentType:
		if remoteHandlerCategory == topologyConfig.ServiceManagerCategory {
			return fmt.Errorf("cannot register manager handler %q as outbound on proxy", outboundURL)
		}
		if handlerCategory == topologyConfig.ServiceManagerCategory {
			return fmt.Errorf("cannot register proxy/independent outbound on proxy manager handler")
		}
		if err := m.proxySetupRegisterOutbounds(handlerCategory, endpoint, remotePublicKey, commands, outboundURL, localCmd); err != nil {
			return err
		}
		return m.requireProxyOutboundWhitelist(inboundURL, secret)

	case topologyConfig.ExtensionType:
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

		if handlerCategory == topologyConfig.ServiceManagerCategory {
			control, err := m.proxyManagerControlClient()
			if err != nil {
				return err
			}
			defer func() { _ = control.Close() }()

			if err := control.RegisterOutbounds(endpoint, remotePublicKey, commands, outboundURL, localCmd); err != nil {
				return fmt.Errorf("control.RegisterOutbounds(%q): %w", outboundURL, err)
			}
			return nil
		}

		if err := m.proxySetupRegisterOutbounds(handlerCategory, endpoint, remotePublicKey, commands, outboundURL, localCmd); err != nil {
			return err
		}
		return m.requireProxyOutboundWhitelist(inboundURL, secret)

	default:
		return fmt.Errorf("unsupported outbound service type %q for %q", remoteService.Type, outboundURL)
	}
}

func (m *ProxyManager) onSecureInbounds(req message.RequestInterface) message.ReplyInterface {
	secret, err := req.RouteParameters().StringValue("manager-hmac-secret")
	if err != nil || secret == "" {
		secret, err = req.RouteParameters().StringValue("secret")
		if err != nil {
			return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('manager-hmac-secret'): %v", err))
		}
	}
	if secret == "" {
		return req.Fail("manager-hmac-secret is required")
	}

	managerURL, err := req.RouteParameters().StringValue("manager-url")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('manager-url'): %v", err))
	}
	if managerURL == "" {
		return req.Fail("manager-url is required")
	}

	signature, err := req.RouteParameters().StringValue("signature")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('signature'): %v", err))
	}
	if signature == "" {
		return req.Fail("signature is required")
	}

	if m.topology == nil {
		return req.Fail("topology is nil")
	}
	storedPublicKey, err := getPublicKeyFromConfig(managerURL, m.topology)
	if err != nil {
		return req.Fail(err.Error())
	}

	delete(req.RouteParameters(), "signature")
	if err := message.Verify(req.String(), signature, storedPublicKey); err != nil {
		return req.Fail(fmt.Sprintf("message.Verify: %v", err))
	}

	inbounds, err := req.RouteParameters().NestedValue("in-inbounds")
	if err != nil {
		inbounds, err = req.RouteParameters().NestedValue("inbounds")
		if err != nil {
			return req.Fail(fmt.Sprintf("req.RouteParameters().NestedValue('in-inbounds'): %v", err))
		}
	}
	outbounds, err := req.RouteParameters().NestedValue("in-outbounds")
	if err != nil {
		outbounds, err = req.RouteParameters().NestedValue("outbounds")
		if err != nil {
			outbounds = datatype.New()
		}
	}

	m.mu.Lock()
	if _, ok := m.outboundSecrets[managerURL]; ok {
		m.mu.Unlock()
		return req.Fail("manager-url conflicts with outbound")
	}
	m.inboundSecrets[managerURL] = secret
	m.mu.Unlock()
	if err := m.whitelistSecret(secret); err != nil {
		return req.Fail(fmt.Sprintf("whitelistSecret: %v", err))
	}

	selfService, err := m.proxySelfService()
	if err != nil {
		return req.Fail(fmt.Sprintf("proxySelfService: %v", err))
	}
	controlTimeout := handlerControlTimeout(selfService)

	replyInbounds := make(map[string]string, len(inbounds))
	for depRoute, raw := range inbounds {
		cred, ok := routeCredentialFromRaw(raw)
		if !ok {
			continue
		}
		routeURL, err := mushroom.Parse(depRoute)
		if err != nil {
			return req.Fail(fmt.Sprintf("mushroom.Parse(%q): %v", depRoute, err))
		}
		pubKey, err := m.secureInbound(routeURL, cred.Secret, controlTimeout)
		if err != nil {
			return req.Fail(fmt.Sprintf("secureInbound(%q): %v", depRoute, err))
		}
		if err := m.allowPublicKey(cred.RouteURL, routeURL, cred.PublicKey); err != nil {
			return req.Fail(fmt.Sprintf("allowPublicKey(%q): %v", depRoute, err))
		}
		replyInbounds[depRoute] = pubKey
	}

	replyOutbounds := make(map[string]string, len(outbounds))
	for depRoute, raw := range outbounds {
		cred, ok := routeCredentialFromRaw(raw)
		if !ok || cred.RouteURL == "" {
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

func (m *ProxyManager) onSecureEdges(req message.RequestInterface) message.ReplyInterface {
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

func (m *ProxyManager) allowOutboundManager(depURL string) error {
	depFullURL, err := asTopologyURL(depURL, m.topology)
	if err != nil {
		return fmt.Errorf("asTopologyURL(%q): %w", depURL, err)
	}

	managerLink := depFullURL.As(mushroom.SERVICE).New(topologyConfig.ServiceManagerCategory)
	pubKey, err := getPublicKeyFromConfig(managerLink.String(), m.topology)
	if err != nil {
		return fmt.Errorf("getPublicKeyFromConfig(%q): %w", managerLink.String(), err)
	}
	if pubKey == "" {
		return fmt.Errorf("manager public key is empty for %q", managerLink.String())
	}

	serviceURL, err := m.proxyServiceURL()
	if err != nil {
		return fmt.Errorf("proxyServiceURL: %w", err)
	}
	zap.AuthCurveAdd(serviceURL.As(mushroom.HANDLER).String(), pubKey, managerLink.As(mushroom.HANDLER))
	return nil
}

func (m *ProxyManager) getDepDereferences() (map[string]struct{}, error) {
	if err := m.ensureTopologyClient(); err != nil {
		return nil, err
	}
	link, err := m.topology.GetLink(m.serviceName)
	if err != nil {
		return nil, fmt.Errorf("topology.GetLink(%q): %w", m.serviceName, err)
	}
	mushroomURL, err := mushroom.Parse(link)
	if err != nil {
		return nil, fmt.Errorf("mushroom.Parse(%q): %w", link, err)
	}
	serviceConfig, err := m.topology.Service(mushroomURL.AsDereference().String())
	if err != nil {
		return nil, fmt.Errorf("topology.Service: %w", err)
	}

	depURLs := make(map[string]struct{})
	addHandlerDep := func(u string) error {
		depLink, err := m.topology.GetLink(u)
		if err != nil {
			return fmt.Errorf("topology.GetLink('%s'): %w", u, err)
		}
		depMushroomURL, err := mushroom.Parse(depLink)
		if err != nil {
			return fmt.Errorf("mushroom.Parse(%q): %w", depLink, err)
		}
		derefU := depMushroomURL.AsDereference().String()
		svcDep, err := m.topology.Service(derefU)
		if err == nil && (svcDep.IsIpc() || svcDep.IsInproc()) {
			depURLs[derefU] = struct{}{}
		}
		return nil
	}

	for _, hdep := range serviceConfig.HandlerDeps {
		for _, u := range hdep.Proxies {
			if err := addHandlerDep(u); err != nil {
				return nil, err
			}
		}
		for _, u := range hdep.Extensions {
			if err := addHandlerDep(u); err != nil {
				return nil, err
			}
		}
	}

	return depURLs, nil
}

func (m *ProxyManager) requireProxyOutboundWhitelist(outboundURL, secret string) error {
	params := datatype.New().Set("outbound-url", outboundURL)
	if secret != "" {
		params.Set("secret", secret)
	}

	reply, err := m.proxySetupRoundTrip(&message.Request{
		Command:    handlers.RequireWhitelistCommand,
		Parameters: params,
	})
	if err != nil {
		return err
	}
	if !reply.IsOK() {
		return fmt.Errorf("proxySetup.Receive(%q): %s", handlers.RequireWhitelistCommand, reply.ErrorMessage())
	}

	return nil
}

func (m *ProxyManager) handshakeCallerInboundDep(depURL string, callerInbounds map[string]string) error {
	fullURL, err := asTopologyURL(depURL, m.topology)
	if err != nil {
		return fmt.Errorf("asTopologyURL(%q): %w", depURL, err)
	}

	depServiceURL := fullURL.As(mushroom.SERVICE)

	m.mu.Lock()
	if m.outboundSecrets[depServiceURL.AsDereference().String()] == "" {
		m.outboundSecrets[depServiceURL.AsDereference().String()] = message.GenerateSecret()
	}
	m.mu.Unlock()

	serviceLink, err := m.topology.GetLink(m.serviceName)
	if err != nil {
		return fmt.Errorf("topology.GetLink(%q): %w", m.serviceName, err)
	}
	managerLink, err := mushroom.As(serviceLink, topologyConfig.ServiceManagerCategory)
	if err != nil {
		return fmt.Errorf("mushroom.As(%q, %q): %w", serviceLink, topologyConfig.ServiceManagerCategory, err)
	}

	depService, err := m.topology.Service(depServiceURL.AsDereference().String())
	if err != nil {
		return fmt.Errorf("topology.Service(%q): %w", depServiceURL.AsDereference().String(), err)
	}

	managerHandler, err := depService.HandlerByCategory(topologyConfig.ServiceManagerCategory)
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
	node.Timeout(handshakeRequestTimeout(depService))
	node.Attempt(2)

	selfService, err := m.proxySelfService()
	if err != nil {
		return fmt.Errorf("proxySelfService: %w", err)
	}
	controlTimeout := handlerControlTimeout(selfService)

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
		publicKey, err := m.secureInbound(protectedURL, hmacSecret, controlTimeout)
		if err != nil {
			return fmt.Errorf("secureInbound(%q): %w", protectedRouteURL, err)
		}
		inOutbounds[inboundURL.String()] = RouteCredential{
			RouteURL:  protectedURL.String(),
			PublicKey: publicKey,
			Secret:    hmacSecret,
		}
	}

	hmacSecret := m.outboundSecrets[depServiceURL.AsDereference().String()]
	msg := &message.Request{
		Command: Handshake,
		Parameters: datatype.New().
			Set("manager-hmac-secret", hmacSecret).
			Set("manager-url", managerLink.String()).
			Set("in-inbounds", map[string]RouteCredential{}).
			Set("in-outbounds", inOutbounds),
	}
	signature, err := message.Sign(msg.String(), m.secretKey)
	if err != nil {
		return fmt.Errorf("message.Sign: %w", err)
	}
	msg.Parameters.Set("signature", signature)

	reply, err := node.Request(msg)
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

func (m *ProxyManager) whitelistManagerInDeps(depURL string) error {
	fullURL, err := asTopologyURL(depURL, m.topology)
	if err != nil {
		return fmt.Errorf("asTopologyURL(%q): %w", depURL, err)
	}

	depServiceURL := fullURL.As(mushroom.SERVICE)

	m.mu.Lock()
	secret := m.outboundSecrets[depServiceURL.AsDereference().String()]
	m.mu.Unlock()
	if secret == "" {
		return fmt.Errorf("dep %q has no self-in-deps handshake secret", depURL)
	}

	depService, err := m.topology.Service(depServiceURL.AsDereference().String())
	if err != nil {
		return fmt.Errorf("topology.Service(%q): %w", depServiceURL.AsDereference().String(), err)
	}

	managerHandler, err := depService.HandlerByCategory(topologyConfig.ServiceManagerCategory)
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
	depManagerLink := depServiceURL.New(topologyConfig.ServiceManagerCategory)
	pubKey, err := getPublicKeyFromConfig(depManagerLink.String(), m.topology)
	if err != nil {
		return fmt.Errorf("getPublicKeyFromConfig(%q): %w", depManagerLink.String(), err)
	}
	node.Socket.Allow(pubKey)
	node.Socket.Secure(m.secretKey)
	node.Timeout(handshakeRequestTimeout(depService))
	node.Attempt(1)

	serviceURL, err := m.proxyServiceURL()
	if err != nil {
		return fmt.Errorf("proxyServiceURL: %w", err)
	}

	depOutbounds, err := filterTopologyOutbounds(m.outbounds, depServiceURL, serviceURL, true)
	if err != nil {
		return fmt.Errorf("filterTopologyOutbounds: %w", err)
	}

	outbounds := make(map[string]string, len(depOutbounds))
	for route, outboundURL := range depOutbounds {
		dep := outboundURL.As(mushroom.SERVICE).AsDereference().String()
		depManagerLink := outboundURL.As(mushroom.SERVICE).New(topologyConfig.ServiceManagerCategory)
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

	msg := &message.Request{
		Command: SecureEdges,
		Parameters: datatype.New().
			Set(SecureEdgesProgressParam, SecureEdgesProgressManagerInDeps).
			Set("outbounds", outbounds),
	}

	reply, err := node.Request(msg)
	if err != nil {
		return fmt.Errorf("socket.Request(%q): %w", SecureEdges, err)
	}
	if !reply.IsOK() {
		return fmt.Errorf("reply.Message: %s", reply.ErrorMessage())
	}
	return nil
}

func (m *ProxyManager) whitelistDepsInDeps(depURL string) error {
	fullURL, err := asTopologyURL(depURL, m.topology)
	if err != nil {
		return fmt.Errorf("asTopologyURL(%q): %w", depURL, err)
	}

	depServiceURL := fullURL.As(mushroom.SERVICE)

	m.mu.Lock()
	secret := m.outboundSecrets[depServiceURL.AsDereference().String()]
	m.mu.Unlock()
	if secret == "" {
		return fmt.Errorf("dep %q has no self-in-deps handshake secret", depURL)
	}

	depService, err := m.topology.Service(depServiceURL.AsDereference().String())
	if err != nil {
		return fmt.Errorf("topology.Service(%q): %w", depServiceURL.AsDereference().String(), err)
	}

	managerHandler, err := depService.HandlerByCategory(topologyConfig.ServiceManagerCategory)
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
	depManagerLink := depServiceURL.New(topologyConfig.ServiceManagerCategory)
	pubKey, err := getPublicKeyFromConfig(depManagerLink.String(), m.topology)
	if err != nil {
		return fmt.Errorf("getPublicKeyFromConfig(%q): %w", depManagerLink.String(), err)
	}
	node.Socket.Allow(pubKey)
	node.Socket.Secure(m.secretKey)
	node.Timeout(handshakeRequestTimeout(depService))
	node.Attempt(1)

	serviceURL, err := m.proxyServiceURL()
	if err != nil {
		return fmt.Errorf("proxyServiceURL: %w", err)
	}

	depInbounds, err := filterTopologyInbounds(m.inbounds, depServiceURL, serviceURL, true)
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

	msg := &message.Request{
		Command: SecureEdges,
		Parameters: datatype.New().
			Set(SecureEdgesProgressParam, SecureEdgesProgressDepsInDeps).
			Set("inbounds", inbounds),
	}

	reply, err := node.Request(msg)
	if err != nil {
		return fmt.Errorf("socket.Request(%q): %w", SecureEdges, err)
	}
	if !reply.IsOK() {
		return fmt.Errorf("reply.Message: %s", reply.ErrorMessage())
	}
	return nil
}

func (m *ProxyManager) onSetProxyHandler(req message.RequestInterface) message.ReplyInterface {
	if _, err := req.RouteParameters().NestedValue("config"); err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().NestedValue('config'): %v", err))
	}
	return m.forwardProxyHandlerRequest(req, handlers.SetProxyHandlerCommand, false)
}

func (m *ProxyManager) onIsProxyHandlerExist(req message.RequestInterface) message.ReplyInterface {
	return m.forwardProxyHandlerRequest(req, handlers.IsProxyHandlerExistCommand, true)
}

func (m *ProxyManager) onProxyHandlerRunning(req message.RequestInterface) message.ReplyInterface {
	return m.forwardProxyHandlerRequest(req, handlers.IsProxyHandlerRunningCommand, true)
}

func (m *ProxyManager) onStartProxyHandler(req message.RequestInterface) message.ReplyInterface {
	return m.forwardProxyHandlerRequest(req, handlers.StartProxyHandlerCommand, true)
}

func (m *ProxyManager) onStopProxyHandler(req message.RequestInterface) message.ReplyInterface {
	return m.forwardProxyHandlerRequest(req, handlers.StopProxyHandlerCommand, true)
}

func (m *ProxyManager) onRemoveProxyHandler(req message.RequestInterface) message.ReplyInterface {
	return m.forwardProxyHandlerRequest(req, handlers.RemoveProxyHandlerCommand, true)
}

func (m *ProxyManager) getOutboundHmacSecret(depServiceURL string) string {
	return ""
}

func (m *ProxyManager) isOutboundHmacExist(managerURL string) bool {
	return false
}

func (m *ProxyManager) registerHandlerOutbounds(handlerCategory string, endpoint message.Endpoint, publicKey string, cmd string, secret string, outboundURL string, localCmd string, controlTimeout time.Duration) error {
	return nil
}

func (m *ProxyManager) requireHandlerSecure(handlerCategory string, controlTimeout time.Duration) (string, error) {
	return "", nil
}

func (m *ProxyManager) requireHandlerSecureOutbound(handlerCategory string, controlTimeout time.Duration) (string, error) {
	return "", nil
}

func (m *ProxyManager) requireHandlerWhitelist(handlerCategory string, cmd string, secret string, controlTimeout time.Duration) error {
	return nil
}

func (m *ProxyManager) selfService() (config.Service, error) {
	return config.Service{}, nil
}

func (m *ProxyManager) setInboundHmacSecret(managerURL string, secret string) {

}

func (m *ProxyManager) setOutboundHmacSecret(depServiceURL string, secret string) {
}

func (m *ProxyManager) forwardProxyHandlerRequest(req message.RequestInterface, command string, requireCategory bool) message.ReplyInterface {
	serviceName, err := req.RouteParameters().StringValue("service")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('service'): %v", err))
	}
	if serviceName != m.serviceName {
		return req.Fail(fmt.Sprintf("service %q does not match proxy service %q", serviceName, m.serviceName))
	}
	if requireCategory {
		if _, err := req.RouteParameters().StringValue("category"); err != nil {
			return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('category'): %v", err))
		}
	}
	if err := m.ensureProxyHandlersClient(); err != nil {
		return req.Fail(err.Error())
	}

	reply, err := m.proxySetupRoundTrip(&message.Request{
		Command:    command,
		Parameters: req.RouteParameters(),
	})
	if err != nil {
		return req.Fail(fmt.Sprintf("proxyHandlersClient.Request('%s'): %v", command, err))
	}
	return reply
}

func (m *ProxyManager) ensureTopologyClient() error {
	if m.topology != nil {
		return nil
	}
	topologyClient, err := topology.NewClient()
	if err != nil {
		return fmt.Errorf("topology.NewClient: %w", err)
	}
	m.topology = topologyClient
	return nil
}

func (m *ProxyManager) ensureProxyHandlersClient() error {
	if m.setup != nil {
		return nil
	}
	proxyHandlersClient, err := protocolClient.NewPair(m.serviceName+handlers.ProxyHandlersCategory, 0)
	if err != nil {
		return fmt.Errorf("client.NewPair('%s'): %w", m.serviceName+handlers.ProxyHandlersCategory, err)
	}
	m.setup = proxyHandlersClient
	return nil
}

func (m *ProxyManager) proxySetupRoundTrip(req *message.Request) (message.ReplyInterface, error) {
	m.setupMu.Lock()
	defer m.setupMu.Unlock()

	if err := m.ensureProxyHandlersClient(); err != nil {
		return nil, err
	}
	if err := m.setup.Send(req); err != nil {
		return nil, fmt.Errorf("proxySetup.Send(%q): %w", req.Command, err)
	}
	reply := <-m.setup.Receive()
	if reply == nil {
		return nil, fmt.Errorf("proxySetup.Receive(%q): no reply", req.Command)
	}
	return reply, nil
}

func (m *ProxyManager) setProxyHandlers() error {
	if m.topology == nil {
		return fmt.Errorf("topologyClient is nil")
	}
	if m.setup == nil {
		return fmt.Errorf("proxyHandlersClient is nil")
	}

	serviceConfig, err := m.topology.Service(m.serviceName)
	if err != nil {
		return fmt.Errorf("topologyClient.Service('%s'): %w", m.serviceName, err)
	}
	if serviceConfig.Type != topologyConfig.ProxyType {
		return fmt.Errorf("service %q type is %q, expected %q", m.serviceName, serviceConfig.Type, topologyConfig.ProxyType)
	}

	for _, variant := range serviceConfig.Handlers {
		proxyHandler, ok := variant.AsProxyHandler()
		if !ok {
			continue
		}
		if len(proxyHandler.Outbounds) == 0 {
			m.warnProxyHandlerNoOutbounds(proxyHandler)
		}

		if err := m.setProxyHandler(proxyHandler); err != nil {
			return fmt.Errorf("setProxyHandler('%s'): %w", proxyHandler.Category, err)
		}
	}

	return nil
}

func (m *ProxyManager) warnProxyHandlerNoOutbounds(proxyHandler topologyConfig.ProxyHandler) {
	if m.logger == nil {
		fmt.Printf("warning: proxy %q has no outbounds yet; forwarding will fail until they are configured\n", proxyHandler.Category)
		return
	}
	m.logger.Warn(
		"proxy has no outbounds yet; forwarding will fail until they are configured",
		"category", proxyHandler.Category,
	)
}

func (m *ProxyManager) setProxyHandler(proxyHandler topologyConfig.ProxyHandler) error {
	if m.setup == nil {
		return fmt.Errorf("proxyHandlersClient is nil")
	}
	if proxyHandler.Outbounds == nil {
		proxyHandler.Outbounds = []string{}
	}
	configParams, err := datatype.NewFromInterface(proxyHandler)
	if err != nil {
		return fmt.Errorf("datatype.NewFromInterface: %w", err)
	}

	reply, err := m.proxySetupRoundTrip(&message.Request{
		Command: handlers.SetProxyHandlerCommand,
		Parameters: datatype.New().
			Set("config", configParams),
	})
	if err != nil {
		return fmt.Errorf("proxyHandlersClient.Send('%s'): %w", handlers.SetProxyHandlerCommand, err)
	}
	if !reply.IsOK() {
		return fmt.Errorf("proxyHandlersClient.Receive('%s'): %s", handlers.SetProxyHandlerCommand, reply.ErrorMessage())
	}
	return nil
}

func (m *ProxyManager) getCurveSecret() string {
	return m.secretKey
}

func (m *ProxyManager) proxyHandlersRequest(command string) error {
	reply, err := m.proxySetupRoundTrip(&message.Request{
		Command:    command,
		Parameters: datatype.New(),
	})
	if err != nil {
		return fmt.Errorf("proxyHandlersClient.Send('%s'): %w", command, err)
	}
	if !reply.IsOK() {
		return fmt.Errorf("proxyHandlersClient.Receive('%s'): %s", command, reply.ErrorMessage())
	}
	return nil
}

// Start the orchestra in the background.
// If it failed to run, then return an error.
// The url request is the main service to which this orchestra belongs too.
func (m *ProxyManager) Start() error {
	if !m.Interface.IsRouteExist(IsServiceRunning) {
		if err := m.Interface.Route(IsServiceRunning, m.onIsServiceRunning); err != nil {
			return fmt.Errorf(`handler.Route("%s"): %w`, IsServiceRunning, err)
		}
	}
	if !m.Interface.IsRouteExist(StartService) {
		if err := m.Interface.Route(StartService, m.onStartService); err != nil {
			return fmt.Errorf(`handler.Route("%s"): %w`, StartService, err)
		}
	}
	if !m.Interface.IsRouteExist(StopService) {
		if err := m.Interface.Route(StopService, m.onStopService); err != nil {
			return fmt.Errorf(`handler.Route("%s"): %w`, StopService, err)
		}
	}
	if !m.Interface.IsRouteExist(Services) {
		if err := m.Interface.Route(Services, m.onServices); err != nil {
			return fmt.Errorf(`handler.Route("%s"): %w`, Services, err)
		}
	}
	if !m.Interface.IsRouteExist(SecureEdges) {
		if err := m.Interface.Route(SecureEdges, m.onSecureEdges); err != nil {
			return fmt.Errorf(`handler.Route("%s"): %w`, SecureEdges, err)
		}
	}
	if !m.Interface.IsRouteExist(handlers.SetProxyHandlerCommand) {
		if err := m.Interface.Route(handlers.SetProxyHandlerCommand, m.onSetProxyHandler); err != nil {
			return fmt.Errorf(`handler.Route("%s"): %w`, handlers.SetProxyHandlerCommand, err)
		}
	}
	if !m.Interface.IsRouteExist(handlers.IsProxyHandlerExistCommand) {
		if err := m.Interface.Route(handlers.IsProxyHandlerExistCommand, m.onIsProxyHandlerExist); err != nil {
			return fmt.Errorf(`handler.Route("%s"): %w`, handlers.IsProxyHandlerExistCommand, err)
		}
	}
	if !m.Interface.IsRouteExist(handlers.IsProxyHandlerRunningCommand) {
		if err := m.Interface.Route(handlers.IsProxyHandlerRunningCommand, m.onProxyHandlerRunning); err != nil {
			return fmt.Errorf(`handler.Route("%s"): %w`, handlers.IsProxyHandlerRunningCommand, err)
		}
	}
	if !m.Interface.IsRouteExist(handlers.StartProxyHandlerCommand) {
		if err := m.Interface.Route(handlers.StartProxyHandlerCommand, m.onStartProxyHandler); err != nil {
			return fmt.Errorf(`handler.Route("%s"): %w`, handlers.StartProxyHandlerCommand, err)
		}
	}
	if !m.Interface.IsRouteExist(handlers.StopProxyHandlerCommand) {
		if err := m.Interface.Route(handlers.StopProxyHandlerCommand, m.onStopProxyHandler); err != nil {
			return fmt.Errorf(`handler.Route("%s"): %w`, handlers.StopProxyHandlerCommand, err)
		}
	}
	if !m.Interface.IsRouteExist(handlers.RemoveProxyHandlerCommand) {
		if err := m.Interface.Route(handlers.RemoveProxyHandlerCommand, m.onRemoveProxyHandler); err != nil {
			return fmt.Errorf(`handler.Route("%s"): %w`, handlers.RemoveProxyHandlerCommand, err)
		}
	}

	if err := m.ensureProxyHandlersClient(); err != nil {
		return fmt.Errorf("ensureProxyHandlersClient: %w", err)
	}
	if err := m.setProxyHandlers(); err != nil {
		return fmt.Errorf("setProxyHandlers: %w", err)
	}
	if err := m.proxyHandlersRequest(handlers.StartProxyHandlersCommand); err != nil {
		return err
	}

	serviceLink, err := m.topology.GetLink(m.serviceName)
	if err != nil {
		return fmt.Errorf("topology.GetLink(%q): %w", m.serviceName, err)
	}

	handlerLink, err := mushroom.As(serviceLink, topologyConfig.ServiceManagerCategory)
	if err != nil {
		return fmt.Errorf("mushroom.As(%q, %q): %w", serviceLink, topologyConfig.ServiceManagerCategory, err)
	}
	m.Interface.SetMushroomURL(handlerLink.String())

	m.Interface.Secure(m.secretKey)
	zap.AuthDynamicAllow(handlerLink.String())

	nodeHandshake, err := NewHandshake(handlerLink, m.topology, m)
	if err != nil {
		return fmt.Errorf("NewHandshake: %w", err)
	}
	m.NodeHandshake = nodeHandshake
	if !m.Interface.IsRouteExist(Handshake) {
		if err := m.Interface.Route(Handshake, m.NodeHandshake.onHandshake); err != nil {
			return fmt.Errorf(`handler.Route("%s"): %w`, Handshake, err)
		}
	}
	if err := m.NodeHandshake.start(); err != nil {
		return fmt.Errorf("NodeHandshake.start: %w", err)
	}

	if err := m.Interface.Start(); err != nil {
		return fmt.Errorf("handler.Start: %w", err)
	}
	m.NodeHandshake.Print()

	fmt.Printf("ProxyManager.Start: %v\n", m.Interface.MushroomURL())

	m.running = true

	return nil
}
