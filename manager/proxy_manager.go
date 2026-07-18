package manager

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/log"
	protocolClient "github.com/noPerfection/protocol/client"
	protocolHandler "github.com/noPerfection/protocol/handler"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service/handlers"
	"github.com/noPerfection/service/mushroom"
	"github.com/noPerfection/topology"
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
	serviceName string
	topology    *topology.Client
	handlers    *protocolClient.SyncReplierClient
	blocker     **sync.WaitGroup
	running     bool
	logger      *log.Logger
	secretKey   string
	pubKey      string
	mu          sync.Mutex
	inbounds    map[string]string
	outbounds   map[string]string
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

	proxyHandlersClient, err := protocolClient.NewSyncReplier(serviceName+handlers.ProxyHandlersCategory, 0)
	if err != nil {
		_ = topologyClient.Close()
		return nil, fmt.Errorf("client.NewSyncReplier('%s'): %w", serviceName+handlers.ProxyHandlersCategory, err)
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

	handler := protocolHandler.NewReplier()

	h := &ProxyManager{
		Interface:   handler,
		topology:    topologyClient,
		handlers:    proxyHandlersClient,
		serviceName: serviceName,
		secretKey:   sec,
		pubKey:      pub,
		inbounds:    make(map[string]string),
		outbounds:   make(map[string]string),
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

// SetLogger sets the optional logger for the proxy manager.
func (m *ProxyManager) SetLogger(logger *log.Logger) error {
	m.logger = logger
	if m.Interface == nil {
		return nil
	}
	if err := m.Interface.SetLogger(logger); err != nil {
		return fmt.Errorf("manager SetLogger: %w", err)
	}
	return nil
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
	key, err := outboundDereferenceKey(serviceURL, m.topology)
	if err != nil {
		return ""
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	return m.outbounds[key]
}

// For now, lets just return manager.running.
func (m *ProxyManager) IsServiceRunning(serviceName string, attempts ...int) (bool, error) {
	if serviceName == "" || serviceName == m.serviceName {
		return m.running, nil
	}
	if err := m.ensureTopologyClient(); err != nil {
		return false, err
	}
	return isServiceRunningWithReload(m.topology, serviceName, m.secretKey, m.getHmacSecret(serviceName), attempts...)
}

func (m *ProxyManager) StopService(serviceName string) error {
	if serviceName == "" || serviceName == m.serviceName {
		if err := m.proxyHandlersRequest(handlers.StopProxyHandlersCommand); err != nil {
			return err
		}
		if err := m.handlers.Close(); err != nil {
			return fmt.Errorf("proxyHandlersClient.Close: %w", err)
		}
		m.handlers = nil
		if m.topology != nil {
			if err := m.topology.Close(); err != nil {
				return fmt.Errorf("topologyClient.Close: %w", err)
			}
			m.topology = nil
		}

		if m.running && m.blocker != nil && *m.blocker != nil {
			(*m.blocker).Done()
		}
		m.running = false

		return nil
	}
	if err := m.ensureTopologyClient(); err != nil {
		return err
	}
	if err := stopRemoteService(m.topology, serviceName, m.secretKey, m.getHmacSecret(serviceName)); err != nil {
		if localErr := m.topology.StopService(serviceName); localErr == nil {
			return nil
		}
		return fmt.Errorf("stopRemoteService(%q): %w", serviceName, err)
	}
	return m.topology.StopService(serviceName)
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
	if err := handlers.CloseViaControl(m.Interface); err != nil {
		return fmt.Errorf("manager handler close: %w", err)
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

func (m *ProxyManager) onHandshake(req message.RequestInterface) message.ReplyInterface {
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

	if err := m.whitelistSecret(secret); err != nil {
		return req.Fail(fmt.Sprintf("whitelistSecret: %v", err))
	}

	return req.Ok(datatype.New())
}

func (m *ProxyManager) getDepURLs() (map[string]struct{}, error) {
	if err := m.ensureTopologyClient(); err != nil {
		return nil, err
	}
	link, err := m.topology.GetLink(m.serviceName)
	if err != nil {
		return nil, fmt.Errorf("topology.GetLink(%q): %w", m.serviceName, err)
	}
	mushroomURL, err := mushroom.New(link)
	if err != nil {
		return nil, fmt.Errorf("mushroom.New(%q): %w", link, err)
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
		depMushroomURL, err := mushroom.New(depLink)
		if err != nil {
			return fmt.Errorf("mushroom.New(%q): %w", depLink, err)
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

func (m *ProxyManager) handshakeOutbound(depURL string) error {
	secret := m.secretKey

	key, err := outboundDereferenceKey(depURL, m.topology)
	if err != nil {
		return fmt.Errorf("outboundDereferenceKey(%q): %w", depURL, err)
	}

	m.mu.Lock()
	m.outbounds[key] = secret
	m.mu.Unlock()

	serviceLink, err := m.topology.GetLink(m.serviceName)
	if err != nil {
		return fmt.Errorf("topology.GetLink(%q): %w", m.serviceName, err)
	}
	managerLink, err := mushroom.New(serviceLink, topologyConfig.ServiceManagerCategory)
	if err != nil {
		return fmt.Errorf("mushroom.New(%q, %q): %w", serviceLink, topologyConfig.ServiceManagerCategory, err)
	}
	inboundURL := managerLink.String()

	depService, err := m.topology.Service(depURL)
	if err != nil {
		return fmt.Errorf("topology.Service(%q): %w", depURL, err)
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

// Handshake waits for IPC/inproc handler deps to become running and exchanges HMAC secrets.
func (m *ProxyManager) Handshake() error {
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
			running, runErr := m.IsServiceRunning(depURL, attempts)
			if runErr != nil {
				if errors.Is(runErr, message.ErrAccessDenied) {
					if err := m.handshakeOutbound(depURL); err != nil {
						errCh <- fmt.Errorf("handshakeOutbound(%q): %w", depURL, err)
						return
					}
					running, runErr = m.IsServiceRunning(depURL, attempts)
				}
				if runErr != nil {
					errCh <- fmt.Errorf("IsServiceRunning(%q, attempts: %d): %w", depURL, attempts, runErr)
					return
				}
			}
			if running {
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

	reply, err := m.handlers.Request(&message.Request{
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
	if m.handlers != nil {
		return nil
	}
	proxyHandlersClient, err := protocolClient.NewSyncReplier(m.serviceName+handlers.ProxyHandlersCategory, 0)
	if err != nil {
		return fmt.Errorf("client.NewSyncReplier('%s'): %w", m.serviceName+handlers.ProxyHandlersCategory, err)
	}
	m.handlers = proxyHandlersClient
	return nil
}

func (m *ProxyManager) setProxyHandlers() error {
	if m.topology == nil {
		return fmt.Errorf("topologyClient is nil")
	}
	if m.handlers == nil {
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
			continue
		}

		if err := m.setProxyHandler(proxyHandler); err != nil {
			return fmt.Errorf("setProxyHandler('%s'): %w", proxyHandler.Category, err)
		}
	}

	return nil
}

func (m *ProxyManager) warnProxyHandlerNoOutbounds(proxyHandler topologyConfig.ProxyHandler) {
	if m.logger == nil {
		fmt.Printf("warning: proxy %q has no outbounds, please set it before starting the proxy, it wont be started yet\n", proxyHandler.Category)
		return
	}
	m.logger.Warn(
		"proxy has no outbounds, please set it before starting the proxy, it wont be started yet",
		"category", proxyHandler.Category,
	)
}

func (m *ProxyManager) setProxyHandler(proxyHandler topologyConfig.ProxyHandler) error {
	if m.handlers == nil {
		return fmt.Errorf("proxyHandlersClient is nil")
	}
	configParams, err := datatype.NewFromInterface(proxyHandler)
	if err != nil {
		return fmt.Errorf("datatype.NewFromInterface: %w", err)
	}

	reply, err := m.handlers.Request(&message.Request{
		Command: handlers.SetProxyHandlerCommand,
		Parameters: datatype.New().
			Set("config", configParams),
	})
	if err != nil {
		return fmt.Errorf("proxyHandlersClient.Request('%s'): %w", handlers.SetProxyHandlerCommand, err)
	}
	if !reply.IsOK() {
		return fmt.Errorf("proxyHandlersClient.Request('%s'): %s", handlers.SetProxyHandlerCommand, reply.ErrorMessage())
	}
	return nil
}

func (m *ProxyManager) proxyHandlersRequest(command string) error {
	if m.handlers == nil {
		return fmt.Errorf("proxyHandlersClient is nil")
	}
	reply, err := m.handlers.Request(&message.Request{
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
	if !m.Interface.IsRouteExist(Handshake) {
		if err := m.Interface.Route(Handshake, m.onHandshake); err != nil {
			return fmt.Errorf(`handler.Route("%s"): %w`, Handshake, err)
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

	handlerLink, err := mushroom.New(serviceLink, topologyConfig.ServiceManagerCategory)
	if err != nil {
		return fmt.Errorf("handlers.AsHandlerLink(%q): %w", topologyConfig.ServiceManagerCategory, err)
	}
	m.Interface.SetMushroomURL(handlerLink.String())

	m.Interface.Secure(m.secretKey)

	if err := m.Interface.Start(); err != nil {
		return fmt.Errorf("handler.Start: %w", err)
	}

	m.running = true

	return nil
}
