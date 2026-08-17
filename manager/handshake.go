package manager

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/log"
	protocolClient "github.com/noPerfection/protocol/client"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service/mushroom"
	"github.com/noPerfection/service/zap"
	"github.com/noPerfection/topology"
	"github.com/noPerfection/topology/config"
)

type ManagerInterface interface {
	routeInboundsTopology() *topology.Client
	routeInboundsServiceDeref() (string, error)
	routeInboundsMushroomURL() (mushroom.TopologyURL, error)
	routeHandlerCommands(category string) ([]string, error)

	isOutboundHmacExist(managerURL string) bool
	// setOutboundHmacSecret(depServiceURL string, secret string)
	getOutboundHmacSecret(depServiceURL string) string

	registerOutboundContext(inboundURL, outboundURL mushroom.TopologyURL, secret, remotePublicKey string) (string, error)

	// handler related
	requireHandlerSecure(handlerCategory string, controlTimeout time.Duration) (string, error)
	requireHandlerWhitelist(handlerCategory string, cmd string, secret string, controlTimeout time.Duration) error
	requireHandlerSecureOutbound(handlerCategory string, controlTimeout time.Duration) (string, error)
	registerHandlerOutbounds(handlerCategory string, endpoint message.Endpoint, publicKey string, cmd string, secret string, outboundURL string, localCmd string, controlTimeout time.Duration) error

	Whitelist(cmd string, secrets ...string) error

	getCurveSecret() string
	selfService() (config.Service, error)
	PublicKey() string
	IsServiceRunning(serviceURL string, attempts ...int) (bool, error)
}

// The NodeHandshake keeps all necessary parameters of the service.
// Manage this service from other parts.
type NodeHandshake struct {
	serviceURL       mushroom.TopologyURL
	topology         *topology.Client
	logger           *log.Logger
	managerSelf      ManagerInterface
	topologyContexts Context
	publicKeys       map[string]string // service.category -> public key

	// handshaked              bool
	handshakeStop           chan struct{}
	handshakeDone           sync.WaitGroup
	handshakeBackgroundOnce sync.Once
	handshakeInterval       time.Duration
	handshakeMu             sync.Mutex
}

// New creates a manager for an independent service.
// serviceURL is the mushroomURL used to locate this service in the topology mycelium
// (a plain symbol such as "main", or a full dereference URL).
// managerEndpoint is the socket other processes use to start, stop, and probe this service.
// New creates a manager for an independent service.
// An optional secretKey may be provided; if given, the public key is derived from it.
// If omitted, a fresh CURVE keypair is generated.
func NewHandshake(serviceURL mushroom.TopologyURL, topology *topology.Client, managerSelf ManagerInterface) (*NodeHandshake, error) {
	if !serviceURL.IsHandlerExist() {
		return nil, fmt.Errorf("serviceURL(%q) must include a handler category", serviceURL)
	}
	if serviceURL.HandlerCategory() != config.ServiceManagerCategory {
		return nil, fmt.Errorf("serviceURL handler category must be %q, got %q", config.ServiceManagerCategory, serviceURL.HandlerCategory())
	}
	h := &NodeHandshake{
		serviceURL: serviceURL,
		// 		handshaked:        false,
		handshakeInterval: defaultHandshakeInterval,
		topologyContexts:  make(Context),
		publicKeys:        make(map[string]string),
		topology:          topology,
		managerSelf:       managerSelf,
	}
	h.publicKeys[serviceURL.As(mushroom.HANDLER).String()] = h.managerSelf.PublicKey()

	return h, nil
}

func (h *NodeHandshake) SetLogger(parent *log.Logger) {
	if parent == nil {
		h.logger = nil
	} else {
		h.logger = parent.Child("NodeHandshake")
	}
}

func (h *NodeHandshake) Print() {
	if len(h.topologyContexts) == 0 {
		return
	}
	fmt.Printf("Context(serviceURL: %q):\n", h.serviceURL.String())
	h.topologyContexts.Print()
}

func (h *NodeHandshake) start() error {
	inbounds, err := getRouteInbounds(h.managerSelf)
	if err != nil {
		return fmt.Errorf("getRouteInbounds: %w", err)
	}

	depDerefs, err := getDepDereferences(h.serviceURL, h.managerSelf, h.topology)
	if err != nil {
		return fmt.Errorf("getDepDereferences: %w", err)
	}

	outbounds, err := buildTopologyOutbounds(inbounds, depDerefs, h.serviceURL.As(mushroom.SERVICE).AsDereference().String())
	if err != nil {
		return fmt.Errorf("buildWhitelistedOutbounds: %w", err)
	}

	if len(depDerefs) > 0 {
		contextURL := h.serviceURL.String()

		context := make(Context)
		context[contextURL] = make(map[string]*ServiceStatus)
		for depDeref := range depDerefs {
			depURL, err := mushroom.Parse(depDeref)
			if err != nil {
				return fmt.Errorf("mushroom.Parse(%q): %w", depDeref, err)
			}
			depURL = depURL.New(config.ServiceManagerCategory)

			depServiceConfig, err := h.topology.Service(depURL.As(mushroom.SERVICE).AsDereference().String())
			if err != nil {
				return fmt.Errorf("topology.Service(%q): %w", depURL.As(mushroom.SERVICE).AsDereference().String(), err)
			}
			endpoint, err := ManagerEndpointForService(depServiceConfig)
			if err != nil {
				return fmt.Errorf("ManagerEndpointForService(%q): %w", depServiceConfig.Name, err)
			}
			protocol := "inproc"
			if endpoint.IsIpc() {
				protocol = "ipc"
			} else if endpoint.IsRemote() {
				protocol = "tcp"
			}

			context[contextURL][depURL.String()] = &ServiceStatus{
				serviceURL: depURL,
				protocol:   protocol,
				managerCon: &ManagerConnection{
					status: ConnectionStatus{
						status:      CONNECTION_STATUS_NOT_SET,
						updatedTime: time.Now(),
						tick:        time.Now(),
					},
				},
			}
			if depServiceConfig.Parameters != nil {
				if pubKey, ok := depServiceConfig.Parameters[ManagerPublicKeyParam].(string); ok && pubKey != "" {
					h.publicKeys[depURL.String()] = pubKey
				}
			}
			depURLAsInbound := depURL.As(mushroom.SERVICE).String()
			depURLAsOutbound := depURL.As(mushroom.SERVICE).AsDereference().String()

			if inbounds != nil && len(inbounds[depURLAsInbound]) > 0 {
				for route, inboundURLs := range inbounds[depURLAsInbound] {
					inboundURL, err := mushroom.Parse(inboundURLs[0])
					if err != nil {
						return fmt.Errorf("mushroom.Parse(%q): %w", inboundURLs[0], err)
					}
					if inboundURL.As(mushroom.SERVICE).String() == h.serviceURL.As(mushroom.SERVICE).String() {
						if context[contextURL][depURL.String()].inbounds == nil {
							context[contextURL][depURL.String()].inbounds = make(map[string]*InboundConnection)
						}

						context[contextURL][depURL.String()].inbounds[route] = &InboundConnection{
							depURL: inboundURL,
							status: ConnectionStatus{
								status:      CONNECTION_STATUS_NOT_SET,
								updatedTime: time.Now(),
								tick:        time.Now(),
							},
						}
					} else {
						context[contextURL][depURL.String()].inboundCons[route] = &InboundConnection{
							depURL: inboundURL,
							status: ConnectionStatus{
								status:      CONNECTION_STATUS_NOT_SET,
								updatedTime: time.Now(),
								tick:        time.Now(),
							},
						}
						context[contextURL][depURL.String()].inboundEdges[route] = &InboundConnection{
							depURL: inboundURL,
							status: ConnectionStatus{
								status:      CONNECTION_STATUS_NOT_SET,
								updatedTime: time.Now(),
								tick:        time.Now(),
							},
						}
					}
				}
			} else {
				context[contextURL][depURL.String()].inbounds = nil
				context[contextURL][depURL.String()].inboundCons = nil
				context[contextURL][depURL.String()].inboundEdges = nil
			}
			if outbounds != nil && len(outbounds[depURLAsOutbound]) > 0 {
				for route, outboundRaw := range outbounds[depURLAsOutbound] {
					outboundURL, err := mushroom.Parse(outboundRaw)
					if err != nil {
						return fmt.Errorf("mushroom.Parse(%q): %w", outboundURL, err)
					}
					if outboundURL.As(mushroom.SERVICE).String() == h.serviceURL.As(mushroom.SERVICE).String() {
						if context[contextURL][depURL.String()].outbounds == nil {
							context[contextURL][depURL.String()].outbounds = make(map[string]*OutboundConnection)
						}
						outboundConnection := OutboundConnection{
							depURL: outboundURL,
							status: ConnectionStatus{
								status:      CONNECTION_STATUS_NOT_SET,
								updatedTime: time.Now(),
								tick:        time.Now(),
							},
						}

						context[contextURL][depURL.String()].outbounds[route] = &outboundConnection
						err = h.prepareOutbound(contextURL, route, &outboundConnection)
						if err != nil {
							fmt.Errorf("NodeHandshake.prepareOutbound(contextURL: %s, routeURL: %s): %w", contextURL, route, err)
						}
					} else {
						context[contextURL][depURL.String()].outboundCons[route] = &OutboundConnection{
							depURL: outboundURL,
							status: ConnectionStatus{
								status:      CONNECTION_STATUS_NOT_SET,
								updatedTime: time.Now(),
								tick:        time.Now(),
							},
						}
						context[contextURL][depURL.String()].outboundEdges[route] = &OutboundConnection{
							depURL: outboundURL,
							status: ConnectionStatus{
								status:      CONNECTION_STATUS_NOT_SET,
								updatedTime: time.Now(),
								tick:        time.Now(),
							},
						}
					}
				}
			} else {
				context[contextURL][depURL.String()].outbounds = nil
				context[contextURL][depURL.String()].outboundCons = nil
				context[contextURL][depURL.String()].outboundEdges = nil
			}
		}

		h.topologyContexts = context
	} else {
		h.topologyContexts = nil
	}

	return nil
}

func (m *NodeHandshake) maybeStartBackgroundHandshake() {
	if len(m.topologyContexts) == 0 {
		return
	}
	m.handshakeBackgroundOnce.Do(func() {
		m.startBackgroundHandshake()
	})
}

func (m *NodeHandshake) startBackgroundHandshake() {
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

func (m *NodeHandshake) stopBackgroundHandshake() {
	if m.handshakeStop == nil {
		return
	}
	close(m.handshakeStop)
	m.handshakeStop = nil
}

// returns service urls from request to determine who calls and in what context.
// it also verifies them to be valid url, so expect handler urls as a return.
//
// Return params:
//   - contextUrl
//   - callerUrl
func getServiceUrls(req message.RequestInterface) (mushroom.TopologyURL, mushroom.TopologyURL, error) {
	managerRawURL, err := req.RouteParameters().StringValue("service-url")
	if err != nil {
		return mushroom.TopologyURL{}, mushroom.TopologyURL{}, fmt.Errorf("req.RouteParameters().StringValue('service-url'): %w", err)
	}
	if managerRawURL == "" {
		return mushroom.TopologyURL{}, mushroom.TopologyURL{}, fmt.Errorf("service-url is empty")
	}
	managerURL, err := mushroom.Parse(managerRawURL)
	if err != nil {
		return mushroom.TopologyURL{}, mushroom.TopologyURL{}, fmt.Errorf("service-url: mushroom.Parse(%q): %v", managerRawURL, err)
	}
	if !managerURL.IsHandlerExist() {
		return mushroom.TopologyURL{}, mushroom.TopologyURL{}, fmt.Errorf("service-url must include a handler category")
	}
	if managerURL.HandlerCategory() != config.ServiceManagerCategory {
		return mushroom.TopologyURL{}, mushroom.TopologyURL{}, fmt.Errorf("service-url handler category must be %q", config.ServiceManagerCategory)
	}
	contextRawURL, err := req.RouteParameters().StringValue("context-url")
	if err != nil {
		return mushroom.TopologyURL{}, mushroom.TopologyURL{}, fmt.Errorf("req.RouteParameters().StringValue('context-url'): %v", err)
	}
	if contextRawURL == "" {
		return mushroom.TopologyURL{}, mushroom.TopologyURL{}, fmt.Errorf("context-url is empty")
	}
	contextURL, err := mushroom.Parse(contextRawURL)
	if err != nil {
		return mushroom.TopologyURL{}, mushroom.TopologyURL{}, fmt.Errorf("context-url: mushroom.Parse(%q): %v", contextRawURL, err)
	}
	if !contextURL.IsHandlerExist() {
		return mushroom.TopologyURL{}, mushroom.TopologyURL{}, fmt.Errorf("context-url must include a handler category")
	}
	if contextURL.HandlerCategory() != config.ServiceManagerCategory {
		return mushroom.TopologyURL{}, mushroom.TopologyURL{}, fmt.Errorf("context-url handler category must be %q", config.ServiceManagerCategory)
	}

	return contextURL, managerURL, nil
}

// returns outbound parameters to secure.
//
// Return params:
//   - hmac				-- RouteParameters('outbound-hmac')
//   - dep-public-key	-- RouteParameters('outbound-dep-public-key')
//   - dep-url			-- RouteParameters('outbound-dep-url')
//   - route-url		-- RouteParameters('outbound-route-url')
func getOutboundParams(req message.RequestInterface) (string, string, mushroom.TopologyURL, mushroom.TopologyURL, error) {
	publicKey, err := req.RouteParameters().StringValue("outbound-dep-public-key")
	if err != nil {
		return "", "", mushroom.TopologyURL{}, mushroom.TopologyURL{}, fmt.Errorf("req.RouteParameters().StringValue('outbound-dep-public-key'): %v", err)
	}
	if publicKey == "" {
		return "", "", mushroom.TopologyURL{}, mushroom.TopologyURL{}, fmt.Errorf("outbound-dep-public-key is empty")
	}

	hmac, err := req.RouteParameters().StringValue("outbound-hmac")
	if err != nil {
		return "", "", mushroom.TopologyURL{}, mushroom.TopologyURL{}, fmt.Errorf("req.RouteParameters().StringValue('outbound-hmac'): %v", err)
	}
	if hmac == "" {
		return "", "", mushroom.TopologyURL{}, mushroom.TopologyURL{}, fmt.Errorf("outbound-hmac is empty")
	}

	outboundRawURL, err := req.RouteParameters().StringValue("outbound-dep-url")
	if err != nil {
		return "", "", mushroom.TopologyURL{}, mushroom.TopologyURL{}, fmt.Errorf("req.RouteParameters().StringValue('outbound-dep-url'): %w", err)
	}
	if outboundRawURL == "" {
		return "", "", mushroom.TopologyURL{}, mushroom.TopologyURL{}, fmt.Errorf("outbound-dep-url is empty")
	}
	outboundURL, err := mushroom.Parse(outboundRawURL)
	if err != nil {
		return "", "", mushroom.TopologyURL{}, mushroom.TopologyURL{}, fmt.Errorf("outbound-dep-url: mushroom.Parse(%q): %v", &outboundRawURL, err)
	}

	routeRawURL, err := req.RouteParameters().StringValue("outbound-route-url")
	if err != nil {
		return "", "", mushroom.TopologyURL{}, mushroom.TopologyURL{}, fmt.Errorf("req.RouteParameters().StringValue('outbound-route-url'): %v", err)
	}
	if routeRawURL == "" {
		return "", "", mushroom.TopologyURL{}, mushroom.TopologyURL{}, fmt.Errorf("outbound-route-url is empty")
	}
	routeURL, err := mushroom.Parse(routeRawURL)
	if err != nil {
		return "", "", mushroom.TopologyURL{}, mushroom.TopologyURL{}, fmt.Errorf("outbound-route-url: mushroom.Parse(%q): %v", routeRawURL, err)
	}

	return hmac, publicKey, outboundURL, routeURL, nil

}

func (m *NodeHandshake) onHandshake(req message.RequestInterface) message.ReplyInterface {
	if m.topology == nil {
		return req.Fail("topology is nil")
	}
	_, callerURL, err := getServiceUrls(req)
	if err != nil {
		return req.Fail(fmt.Sprintf("getServiceUrls: %v", err))
	}

	signature, err := req.RouteParameters().StringValue("signature")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('signature'): %v", err))
	}
	if signature == "" {
		return req.Fail("signature is required")
	}

	storedPublicKey, err := getPublicKeyFromConfig(callerURL.String(), m.topology)
	if err != nil {
		return req.Fail(err.Error())
	}

	delete(req.RouteParameters(), "signature")
	if err := message.Verify(req.String(), signature, storedPublicKey); err != nil {
		return req.Fail(fmt.Sprintf("message.Verify: %v", err))
	}

	secret, err := req.RouteParameters().StringValue("hmac-secret")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('hmac-secret'): %v", err))
	}
	if secret == "" {
		return req.Fail("hmac-secret is required")
	}

	// Give access to the manager that handshaked.
	for _, cmd := range proxyManagerWhitelistCommands() {
		if err := m.managerSelf.Whitelist(cmd, secret); err != nil {
			return req.Fail(fmt.Sprintf(`handler.Whitelist("%s"): %v`, cmd, err))
		}
	}

	return req.Ok()
}

func (m *NodeHandshake) onGetServiceStatus(req message.RequestInterface) message.ReplyInterface {
	contextURL, callerURL, err := getServiceUrls(req)
	if err != nil {
		return req.Fail(fmt.Sprintf("getServiceUrls(): %v", err))
	}

	var status ServiceStatusMsg
	statusKV, err := req.RouteParameters().NestedValue("dep-status")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().NestedValue('dep-status'): %v", err))
	}
	if err := statusKV.Interface(&status); err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().NestedValue('dep-status').Interface(&status): %v", err))
	}

	if len(status.Outbounds) > 0 {
		for routeURL, outbound := range status.Outbounds {
			depStatus, ok := m.topologyContexts[contextURL.String()][callerURL.As(mushroom.HANDLER).String()]
			if !ok {
				outbound.Status = CONNECTION_STATUS_NOT_FOUND
			} else {
				inboundStatus, inboundOK := depStatus.inbounds[outbound.DepURL]
				if !inboundOK {
					outbound.Status = CONNECTION_STATUS_NOT_FOUND
				} else {
					if outbound.Status != inboundStatus.status.Get() {
						outbound.Status = CONNECTION_STATUS_MISMATCH
					} else if outbound.Status == CONNECTION_STATUS_CONNECTED {
						if outbound.CacheHash != depStatus.inbounds[outbound.DepURL].cacheHash {
							outbound.Status = CONNECTION_STATUS_MISMATCH
						}
					}
				}
			}
			status.Outbounds[routeURL] = outbound
		}
	}
	// todo implement it
	// calculate the hash if all are connected
	return req.Ok(datatype.New().Set("dep-status", status))
}

// func (m *NodeHandshake) onSecureInbounds(req message.RequestInterface) message.ReplyInterface {
// 	if m.topology == nil {
// 		return req.Fail("topology is nil")
// 	}
// 	managerRawURL, err := req.RouteParameters().StringValue("service-url")
// 	if err != nil {
// 		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('manager-url'): %v", err))
// 	}
// 	if managerRawURL == "" {
// 		return req.Fail("manager-url is required")
// 	}
// 	managerURL, err := mushroom.Parse(managerRawURL)
// 	if err != nil {
// 		return req.Fail(fmt.Sprintf("mushroom.Parse(%q): %v", managerRawURL, err))
// 	}
// 	if !managerURL.IsHandlerExist() {
// 		return req.Fail("manager-url must include a handler category")
// 	}
// 	if managerURL.HandlerCategory() != config.ServiceManagerCategory {
// 		return req.Fail(fmt.Sprintf("manager-url handler category must be %q", config.ServiceManagerCategory))
// 	}

// 	inboundsRaw, err := req.RouteParameters().NestedValue("in-inbounds")
// 	if err != nil {
// 		return req.Fail(fmt.Sprintf("req.RouteParameters().NestedValue('in-inbounds'): %v", err))
// 	}
// 	inbounds := make(map[string]RouteCredential)
// 	if err := inboundsRaw.Interface(&inbounds); err != nil {
// 		return req.Fail(fmt.Sprintf("inboundsRaw.Interface: %v", err))
// 	}

// 	selfService, err := m.managerSelf.selfService()
// 	if err != nil {
// 		return req.Fail(fmt.Sprintf("selfService: %v", err))
// 	}
// 	controlTimeout := handlerControlTimeout(selfService)

// 	replyInbounds := make(map[string]string, len(inbounds))
// 	for depRoute, cred := range inbounds {
// 		routeURL, err := mushroom.Parse(depRoute)
// 		if err != nil {
// 			return req.Fail(fmt.Sprintf("mushroom.Parse(%q): %v", depRoute, err))
// 		}
// 		pubKey, err := m.secureInbound(routeURL, cred.Secret, controlTimeout)
// 		if err != nil {
// 			return req.Fail(fmt.Sprintf("secureInbound(%q): %v", depRoute, err))
// 		}
// 		if cred.PublicKey != "" {
// 			if err := m.allowPublicKey(cred.RouteURL, routeURL, cred.PublicKey); err != nil {
// 				return req.Fail(fmt.Sprintf("allowPublicKey(%q): %v", depRoute, err))
// 			}
// 		}
// 		replyInbounds[depRoute] = pubKey
// 	}

// 	return req.Ok(datatype.New().Set("inbounds", replyInbounds).Set("outbounds", replyOutbounds))
// }

func (m *NodeHandshake) initiateDepServiceStatus(contextURL string, depURL mushroom.TopologyURL) *ServiceStatus {
	if m.topologyContexts == nil {
		m.topologyContexts = Context{}
	}
	if m.topologyContexts[contextURL] == nil {
		m.topologyContexts[contextURL] = make(map[string]*ServiceStatus)
	}
	serviceURL := depURL.String()
	if m.topologyContexts[contextURL][serviceURL] == nil {
		m.topologyContexts[contextURL][serviceURL] = &ServiceStatus{
			serviceURL: depURL,
		}
	}
	return m.topologyContexts[contextURL][serviceURL]
}

func (m *NodeHandshake) onSecureOutbounds(req message.RequestInterface) message.ReplyInterface {
	contextURL, serviceURL, err := getServiceUrls(req)
	if err != nil {
		return req.Fail(fmt.Sprintf("getServiceUrls: %w", err))
	}

	hmac, depPublicKey, depUrl, routeUrl, err := getOutboundParams(req)
	if err != nil {
		return req.Fail(fmt.Sprintf("getOutboundParams: %w", err))
	}

	// route of outbound should be in this service,
	// while dep is some other connection
	if !routeUrl.Equal(m.serviceURL, mushroom.SERVICE) {
		return req.Fail(fmt.Sprintf(`routeUrl("%s").Equal(this.serviceURL: "%s")? false`, routeUrl, m.serviceURL))
	}
	// dep should be the caller serviceURL
	if !depUrl.Equal(serviceURL, mushroom.SERVICE) {
		return req.Fail(fmt.Sprintf(`depURL("%s").Equal(callerURL: "%s")? false`, depUrl, serviceURL))
	}

	m.publicKeys[depUrl.As(mushroom.HANDLER).String()] = depPublicKey

	depStatus := m.initiateDepServiceStatus(contextURL.String(), serviceURL.As(mushroom.HANDLER))
	if depStatus.inbounds == nil {
		depStatus.inbounds = make(map[string]*InboundConnection)
	}

	depStatus.inbounds[depUrl.String()] = &InboundConnection{
		status: NewConnectionStatus(CONNECTION_STATUS_CHECKING),
		depURL: routeUrl,
		hmac:   hmac,
	}

	// 	replyOutbounds := make(map[string]string, len(outbounds))
	pubKey, err := m.managerSelf.registerOutboundContext(routeUrl, depUrl, hmac, depPublicKey)
	if err != nil {
		return req.Fail(fmt.Sprintf("registerOutboundContext(%q): %v", depUrl.String(), err))
	}
	if m.publicKeys[routeUrl.As(mushroom.HANDLER).String()] != pubKey {
		m.publicKeys[routeUrl.As(mushroom.HANDLER).String()] = pubKey
	}
	depStatus.inbounds[depUrl.String()].status.Set(CONNECTION_STATUS_CONNECTED)
	depStatus.inbounds[depUrl.String()].cacheHash = m.computeOutboundCacheHash(routeUrl, depUrl, hmac)

	return req.Ok(datatype.New().Set("outbound-route-public-key", pubKey))

	// return req.Ok(datatype.New().Set("inbounds", replyInbounds).Set("outbounds", replyOutbounds))

	// func (m *NodeHandshake) onSecureEdges(req message.RequestInterface) message.ReplyInterface {
	// 	if m.topology == nil {
	// 		return req.Fail("topology is nil")
	// 	}

	// 	progress, _ := req.RouteParameters().StringValue(SecureEdgesProgressParam)
	// 	if progress == "" {
	// 		return req.Fail("progress is required")
	// 	}

	//		switch progress {
	//		case SecureEdgesProgressManagerInDeps:
	//			outboundsRaw, err := req.RouteParameters().NestedValue("outbounds")
	//			if err != nil {
	//				return req.Fail(fmt.Sprintf("req.RouteParameters().NestedValue('outbounds'): %v", err))
	//			}
	//			outbounds := make(map[string]string)
	//			if err := outboundsRaw.Interface(&outbounds); err != nil {
	//				return req.Fail(fmt.Sprintf("outboundsRaw.Interface: %v", err))
	//			}
	//			for depRoute, depURL := range outbounds {
	//				if depURL == "" {
	//					return req.Fail(fmt.Sprintf("outbound %q has no dep", depRoute))
	//				}
	//				if err := m.allowOutboundManager(depURL); err != nil {
	//					return req.Fail(fmt.Sprintf("allowOutboundManager(%q from %q): %v", depURL, depRoute, err))
	//				}
	//			}
	//			return req.Ok(datatype.New())
	//		case SecureEdgesProgressDepsInDeps:
	//			inboundsRaw, err := req.RouteParameters().NestedValue("inbounds")
	//			if err != nil {
	//				return req.Fail(fmt.Sprintf("req.RouteParameters().NestedValue('inbounds'): %v", err))
	//			}
	//			inbounds := make(map[string]string)
	//			if err := inboundsRaw.Interface(&inbounds); err != nil {
	//				return req.Fail(fmt.Sprintf("inboundsRaw.Interface: %v", err))
	//			}
	//			grouped := make(map[string]map[string]string)
	//			for protectedRouteURL, inboundRouteURL := range inbounds {
	//				inboundURL, err := asTopologyURL(inboundRouteURL, m.topology)
	//				if err != nil {
	//					return req.Fail(fmt.Sprintf("asTopologyURL(%q): %v", inboundRouteURL, err))
	//				}
	//				depURL := inboundURL.As(mushroom.SERVICE).AsDereference().String()
	//				if grouped[depURL] == nil {
	//					grouped[depURL] = make(map[string]string)
	//				}
	//				grouped[depURL][protectedRouteURL] = inboundRouteURL
	//			}
	//			for depURL, callerInbounds := range grouped {
	//				if err := m.handshakeCallerInboundDep(depURL, callerInbounds); err != nil {
	//					return req.Fail(fmt.Sprintf("handshakeCallerInboundDep(%q): %v", depURL, err))
	//				}
	//			}
	//			return req.Ok(datatype.New())
	//		default:
	//			return req.Fail(fmt.Sprintf("unknown secure-edges progress %q", progress))
	//		}
	//	}
}

// func (m *NodeHandshake) allowOutboundManager(outboundRouteURL string) error {
// 	outboundURL, err := mushroom.Parse(outboundRouteURL)
// 	if err != nil {
// 		return fmt.Errorf("mushroom.Parse(%q): %w", outboundRouteURL, err)
// 	}

// 	managerLink := outboundURL.As(mushroom.SERVICE).New(config.ServiceManagerCategory)
// 	pubKey, err := getPublicKeyFromConfig(managerLink.String(), m.topology)
// 	if err != nil {
// 		return fmt.Errorf("getPublicKeyFromConfig(%q): %w", managerLink.String(), err)
// 	}
// 	if pubKey == "" {
// 		return fmt.Errorf("manager public key is empty for %q", managerLink.String())
// 	}

// 	zap.AuthCurveAdd(m.serviceURL.As(mushroom.HANDLER).String(), pubKey, managerLink.As(mushroom.HANDLER))
// 	return nil
// }

// // inbound url within this service is secured. Whitelist is set.
// //
// // Example (dep-service: entrypoint-proxy, this-service: hello-world)
// //
// //	Outbounds[entrypoint-proxy.main.hello] = {
// //	  RouteURL: entrypoint-proxy.main.hello,
// //	  PublicKey: hello-world.main.public-key,
// //	  Secret: hello-world.main.secret,
// //	}
// //
// // Outbounds to this service from dep. On Handshake, the service registers inOutbounds.
// // onHandshake should return the public key of the entrypoint-proxy.main.hello handler.
// // And here it will be recorded as allow via a control.
// func (m *NodeHandshake) secureInbound(inboundURL mushroom.TopologyURL, secret string, controlTimeout time.Duration) (string, error) {
// 	if !inboundURL.IsRouteExist() {
// 		return "", fmt.Errorf("inboundURL.IsRouteExist() is false: %q", inboundURL.String())
// 	}
// 	selfServiceURL, err := asTopologyURL(m.serviceURL.String(), m.topology)
// 	if err != nil {
// 		return "", fmt.Errorf("asTopologyURL(self): %w", err)
// 	}
// 	if !inboundURL.Equal(selfServiceURL, mushroom.SERVICE) {
// 		return "", fmt.Errorf("inbound route %q is not on this service", inboundURL.String())
// 	}

// 	cmd := inboundURL.AdditionalProps["command"]

// 	handlerCategory := inboundURL.HandlerLink().HandlerCategory()
// 	if handlerCategory == config.ServiceManagerCategory {
// 		if err := m.managerSelf.Whitelist(cmd, secret); err != nil {
// 			return "", fmt.Errorf("Interface.Whitelist(%q): %w", cmd, err)
// 		}
// 		return m.managerSelf.PublicKey(), nil
// 	}

// 	publicKey, err := m.managerSelf.requireHandlerSecure(handlerCategory, controlTimeout)
// 	if err != nil {
// 		return "", fmt.Errorf("control.RequireSecure(%q): %w", handlerCategory, err)
// 	}
// 	zap.AuthDynamicAllow(inboundURL.HandlerLink().String())
// 	if err := m.managerSelf.requireHandlerWhitelist(handlerCategory, cmd, secret, controlTimeout); err != nil {
// 		return "", fmt.Errorf("control.RequireWhitelist(%q): %w", cmd, err)
// 	}
// 	return publicKey, nil
// }

// func (m *NodeHandshake) managerControlClient() (*protocolClient.Control, error) {
// 	service, err := m.managerSelf.selfService()
// 	if err != nil {
// 		return nil, fmt.Errorf("selfService: %w", err)
// 	}
// 	managerHandler, err := service.HandlerByCategory(config.ServiceManagerCategory)
// 	if err != nil {
// 		return nil, fmt.Errorf("manager handler: %w", err)
// 	}
// 	ind, ok := managerHandler.AsIndependentHandler()
// 	if !ok {
// 		return nil, fmt.Errorf("manager handler is not independent")
// 	}
// 	controlEndpoint := protocolHandler.NewInternalControlEndpoint(ind.Endpoint)
// 	return protocolClient.NewControl(controlEndpoint.Id, controlEndpoint.Port)
// }

func (m *NodeHandshake) allowPublicKey(inboundURL, routeURL mushroom.TopologyURL, routePublicKey string) error {
	return nil
}

// allowSelfInDep ensures this manager's CURVE public key is listed in dep's
// parameters.allowed so the dep manager handler can authenticate us.
func (m *NodeHandshake) allowSelfInDep(depStatus *ServiceStatus) error {
	if m.topology == nil {
		return fmt.Errorf("topology is nil")
	}
	if m.managerSelf.PublicKey() == "" {
		return fmt.Errorf("manager public key is empty")
	}

	depServiceURL := depStatus.serviceURL.As(mushroom.SERVICE)
	depService, err := m.topology.Service(depServiceURL.AsDereference().String())
	if err != nil {
		return fmt.Errorf("topology.Service(%q): %w", depServiceURL.AsDereference().String(), err)
	}

	// First if public key is not set, set it
	updated := false
	if !mushroom.IsAllowedClientPublicKey(&depService, m.managerSelf.PublicKey(), config.ServiceManagerCategory) {
		mushroom.AddAllowedPublicKey(&depService, m.serviceURL, m.serviceURL.ResourcePublicKey())
		if err := m.topology.SetService(depService); err != nil {
			return fmt.Errorf("topology.SetService(%q): %w", depService.Name, err)
		}
		updated = true
	}

	if depService.Parameters != nil {
		if pubKey, ok := depService.Parameters[ManagerPublicKeyParam].(string); ok && pubKey != "" {
			if m.publicKeys[depStatus.serviceURL.String()] != pubKey {
				m.publicKeys[depStatus.serviceURL.String()] = pubKey

				if depStatus.managerCon.client != nil {
					if err := depStatus.managerCon.client.Close(); err != nil {
						return fmt.Errorf("managerCon.client.Close: %w", err)
					}
				}
				if err := m.setManagerClient(depStatus); err != nil {
					return fmt.Errorf("setManagerClient: %w", err)
				}

				updated = true
			}
		}
	}

	if !updated {
		return fmt.Errorf("the service public key in topology, and dep's public key allowed in managerCon, nothing to update")
	}

	return nil
}

func (m *NodeHandshake) setManagerClient(depStatus *ServiceStatus) error {
	depServiceURL := depStatus.serviceURL.As(mushroom.SERVICE)

	depServiceConfig, err := m.topology.Service(depServiceURL.AsDereference().String())
	if err != nil {
		return fmt.Errorf("topology.Service(%q): %w", depServiceURL.AsDereference().String(), err)
	}
	depManagerEndpoint, err := ManagerEndpointForService(depServiceConfig)
	if err != nil {
		return fmt.Errorf("ManagerEndpointForService(%q): %w", depServiceConfig.Name, err)
	}

	socket, err := protocolClient.New(
		depManagerEndpoint.Id,
		depManagerEndpoint.Port,
		protocolClient.SyncReplierType,
	)
	if err != nil {
		return fmt.Errorf("client.New: %w", err)
	}

	node := &topology.Client{Socket: socket}
	if m.publicKeys[depStatus.serviceURL.String()] != "" {
		node.Socket.Allow(m.publicKeys[depStatus.serviceURL.String()])
	}
	node.Socket.Secure(m.managerSelf.getCurveSecret())
	node.Timeout(handshakeRequestTimeoutByProtocol(depStatus.protocol))
	node.Attempt(1)

	if depStatus.managerCon.hmac != "" {
		for _, cmd := range proxyManagerWhitelistCommands() {
			if err := node.Whitelist(cmd, depStatus.managerCon.hmac); err != nil {
				return fmt.Errorf(`handler.Whitelist("%s"): %v`, cmd, err)
			}
		}
	}

	depStatus.managerCon.client = node

	return nil
}

// whitelistSelfInDeps is the first of handshaking processes:
//
//	manager-to-manager handshake by whitelisting to all dependencies in topology.
func (m *NodeHandshake) whitelistSelfInDeps(contextURL string, depStatus *ServiceStatus) error {
	// set the hmac secret for the manager connection
	if depStatus.managerCon.hmac == "" {
		depStatus.managerCon.hmac = message.GenerateSecret()
	}

	if err := m.setManagerClient(depStatus); err != nil {
		return fmt.Errorf("setManagerClient: %w", err)
	}

	msg := message.Request{
		Command: Handshake,
		Parameters: datatype.New().
			Set("hmac-secret", depStatus.managerCon.hmac).
			Set("service-url", m.serviceURL.String()).
			Set("context-url", contextURL),
	}

	signature, err := message.Sign(msg.String(), m.managerSelf.getCurveSecret())
	if err != nil {
		return fmt.Errorf("message.Sign: %w", err)
	}
	msg.Parameters.Set("signature", signature)

	depStatus.managerCon.client.Timeout(probeTimeout(depStatus.protocol))

	reply, err := depStatus.managerCon.client.Request(&msg)
	if err != nil {
		return fmt.Errorf("socket.Request(%q): %w", Handshake, err)
	}
	if !reply.IsOK() {
		return fmt.Errorf("reply.Message: %s", reply.ErrorMessage())
	}

	return nil
}

func (m *NodeHandshake) secureInbounds(contextURL string, depStatus *ServiceStatus, inboundUpdates map[string]EdgeConnectionMsg) {
	for routeURL, inbound := range depStatus.inbounds {
		if inbound.status.Get() == CONNECTION_STATUS_CHECKING {
			continue
		} else if inboundUpdates != nil && inboundUpdates[routeURL].Status == CONNECTION_STATUS_CONNECTED {
			continue
		} else {
			// go func() {
			// 	if err := m.secureInbound(contextURL, routeURL, depStatus.managerCon, inbound); err != nil {
			// 		if m.logger != nil {
			// 			m.logger.Error("m.secureOutbound", "contextURL", contextURL,
			// 				"routeURL", routeURL, "outboundURL", inbound.depURL.String())
			// 		}
			// 	}
			// }()
		}
	}
}

func (m *NodeHandshake) secureOutbounds(contextURL string, depStatus *ServiceStatus, outboundUpdates map[string]EdgeConnectionMsg) {
	for routeURL, outbound := range depStatus.outbounds {
		if outbound.status.Get() == CONNECTION_STATUS_CHECKING {
			continue
		} else if outboundUpdates != nil && outboundUpdates[routeURL].Status == CONNECTION_STATUS_CONNECTED {
			continue
		} else {
			go func() {
				if err := m.secureOutbound(contextURL, routeURL, depStatus.managerCon, outbound); err != nil {
					if m.logger != nil {
						m.logger.Error("m.secureOutbound", "contextURL", contextURL,
							"routeURL", routeURL, "outboundURL", outbound.depURL.String())
					}
				}
			}()
		}
	}
}

func (m *NodeHandshake) computeOutboundCacheHash(routeURL, depURL mushroom.TopologyURL, hmac string) string {
	routePubKey := m.publicKeys[routeURL.HandlerLink().String()]
	depPubKey := m.publicKeys[depURL.HandlerLink().String()]

	return message.ComputeCacheHash(routeURL.String(), depURL.String(), routePubKey, depPubKey, hmac)
}

// this service as outbound of the dep, it needs to secure its own inbound.
// this one secures and requires security, before handshaking too.
func (m *NodeHandshake) prepareOutbound(contextURL string, routeURL string, outbound *OutboundConnection) error {
	outbound.hmac = message.GenerateSecret()

	// Secure inbound in this service, if its dep's outbound.
	cmd := outbound.depURL.AdditionalProps["command"]
	handlerCategory := outbound.depURL.HandlerLink().HandlerCategory()
	if handlerCategory == config.ServiceManagerCategory {
		if err := m.managerSelf.Whitelist(cmd, outbound.hmac); err != nil {
			return fmt.Errorf("Interface.Whitelist(%q): %w", cmd, err)
		}
	} else {
		publicKey, err := m.managerSelf.requireHandlerSecure(handlerCategory, 100*time.Millisecond)
		if err != nil {
			return fmt.Errorf("control.RequireSecure(%q): %w", handlerCategory, err)
		}
		zap.AuthDynamicAllow(outbound.depURL.HandlerLink().String())
		if err := m.managerSelf.requireHandlerWhitelist(handlerCategory, cmd, outbound.hmac, 100*time.Millisecond); err != nil {
			return fmt.Errorf("control.RequireWhitelist(%q): %w", cmd, err)
		}
		m.publicKeys[outbound.depURL.HandlerLink().String()] = publicKey
	}

	return nil
}

func (m *NodeHandshake) secureOutbound(contextURL string, routeURL string, con *ManagerConnection, outbound *OutboundConnection) error {
	// set the status
	outbound.status.Set(CONNECTION_STATUS_CHECKING)

	// now call the secureOutbounds request.
	req := message.Request{
		Command: SecureOutbounds,
		Parameters: datatype.New().
			Set("context-url", contextURL).
			Set("service-url", m.serviceURL.String()).
			Set("outbound-dep-public-key", m.publicKeys[outbound.depURL.HandlerLink().String()]).
			Set("outbound-hmac", outbound.hmac).
			Set("outbound-route-url", routeURL).
			Set("outbound-dep-url", outbound.depURL.String()),
	}

	rep, err := con.client.Request(&req)
	if err != nil {
		outbound.status.Set(CONNECTION_STATUS_NOT_SET)
		return fmt.Errorf("request(secure-outbounds, route: %s): %w", routeURL, err)
	} else if !rep.IsOK() {
		outbound.status.Set(CONNECTION_STATUS_NOT_SET)
		return fmt.Errorf("request(secure-outbounds, route: %s): reply: %s", routeURL, rep.ErrorMessage())
	}

	outboundRoutePublicKey, err := rep.ReplyParameters().StringValue("outbound-route-public-key")
	if err != nil {
		outbound.status.Set(CONNECTION_STATUS_NOT_SET)
		return fmt.Errorf("request(secure-outbounds, route: %s).ReplyParameters().StringValue('outbound-route-public-key'): %w", routeURL, err)
	}
	routeTopology, _ := mushroom.Parse(routeURL)
	m.publicKeys[routeTopology.As(mushroom.HANDLER).String()] = outboundRoutePublicKey

	zap.AuthCurveAdd(outbound.depURL.As(mushroom.HANDLER).String(), outboundRoutePublicKey, routeTopology.As(mushroom.HANDLER))
	outbound.status.Set(CONNECTION_STATUS_CONNECTED)
	outbound.cacheHash = m.computeOutboundCacheHash(routeTopology, outbound.depURL, outbound.hmac)

	// func (m *NodeHandshake) secureInbounds(depStatus *ServiceStatus) error {
	// 	inboundsAreMe, err := filterTopologyInbounds(m.topologyInbounds, depStatus.serviceURL, m.serviceURL, false)
	// 	if err != nil {
	// 		return fmt.Errorf("filterTopologyInbounds: %w", err)
	// 	}

	// 	inInbounds := make(map[string]RouteCredential)
	// 	for depRouteURL, inboundURL := range inboundsAreMe {
	// 		hmacSecret := message.GenerateSecret()
	// 		publicKey, err := m.prepareOutboundContext(inboundURL)
	// 		if err != nil {
	// 			return fmt.Errorf("prepareInboundCredential: %w", err)
	// 		}
	// 		inInbounds[depRouteURL] = RouteCredential{
	// 			RouteURL:  inboundURL.String(),
	// 			PublicKey: publicKey,
	// 			Secret:    hmacSecret,
	// 		}
	// 	}

	// 	msg := message.Request{
	// 		Command: Handshake,
	// 		Parameters: datatype.New().
	// 			Set("in-inbounds", inInbounds).
	// 	}

	// 	signature, err := message.Sign(msg.String(), m.curveSecretKey)
	// 	if err != nil {
	// 		return fmt.Errorf("message.Sign: %w", err)
	// 	}
	// 	msg.Parameters.Set("signature", signature)

	// 	reply, err := node.Request(&msg)
	// 	if err != nil {
	// 		return fmt.Errorf("socket.Request(%q): %w", Handshake, err)
	// 	}
	// 	if !reply.IsOK() {
	// 		return fmt.Errorf("reply.Message: %s", reply.ErrorMessage())
	// 	}

	// 	replyInboundsKV, err := reply.ReplyParameters().NestedValue("inbounds")
	// 	if err != nil {
	// 		replyInboundsKV = datatype.New()
	// 	}

	// 	for depRouteURL, cred := range inInbounds {
	// 		inboundInMeURL := cred.RouteURL
	// 		depPublicKey, err := replyInboundsKV.StringValue(depRouteURL)
	// 		if err != nil {
	// 			return fmt.Errorf("reply inbounds public key for %q: %w", depRouteURL, err)
	// 		}
	// 		if err := m.registerOutboundContext(cred.RouteURL, depRouteURL, cred.Secret, depPublicKey); err != nil {
	// 			return fmt.Errorf("registerOutboundContext(%q): %w", inboundInMeURL, err)
	// 		}
	// 	}

	return nil
}

// func (m *NodeHandshake) handshakeCallerInboundDep(depURL string, callerInbounds map[string]string) error {
// 	depFullURL, err := asTopologyURL(depURL, m.topology)
// 	if err != nil {
// 		return fmt.Errorf("asTopologyURL(%q): %w", depURL, err)
// 	}

// 	depServiceURL := depFullURL.As(mushroom.SERVICE)
// 	secret := message.GenerateSecret()

// 	m.managerSelf.setOutboundHmacSecret(depServiceURL.AsDereference().String(), secret)

// 	managerLink := m.serviceURL.New(config.ServiceManagerCategory)

// 	depServiceConfig, err := m.topology.Service(depServiceURL.AsDereference().String())
// 	if err != nil {
// 		return fmt.Errorf("topology.Service(%q): %w", depServiceURL.AsDereference().String(), err)
// 	}

// 	depManagerConfig, err := depServiceConfig.HandlerByCategory(config.ServiceManagerCategory)
// 	if err != nil {
// 		return fmt.Errorf("dep %q manager handler: %w", depServiceConfig.Name, err)
// 	}
// 	depManagerAsIndependent, ok := depManagerConfig.AsIndependentHandler()
// 	if !ok {
// 		return fmt.Errorf("dep %q manager handler is invalid", depServiceConfig.Name)
// 	}

// 	socket, err := protocolClient.New(
// 		depManagerAsIndependent.Endpoint.Id,
// 		depManagerAsIndependent.Endpoint.Port,
// 		protocolClient.HandlerType(depManagerAsIndependent.Type),
// 	)
// 	if err != nil {
// 		return fmt.Errorf("client.New: %w", err)
// 	}
// 	defer socket.Close()

// 	node := &topology.Client{Socket: socket}
// 	if depServiceConfig.Parameters != nil {
// 		if pubKey, ok := depServiceConfig.Parameters[ManagerPublicKeyParam].(string); ok && pubKey != "" {
// 			node.Socket.Allow(pubKey)
// 		}
// 	}
// 	node.Socket.Secure(m.curveSecretKey)
// 	node.Timeout(handshakeRequestTimeout(depServiceConfig))
// 	node.Attempt(1)

// 	selfService, err := m.managerSelf.selfService()
// 	if err != nil {
// 		return fmt.Errorf("selfService: %w", err)
// 	}

// 	inOutbounds := make(map[string]RouteCredential)
// 	for protectedRouteURL, inboundRouteURL := range callerInbounds {
// 		protectedURL, err := asTopologyURL(protectedRouteURL, m.topology)
// 		if err != nil {
// 			return fmt.Errorf("asTopologyURL(%q): %w", protectedRouteURL, err)
// 		}
// 		inboundURL, err := asTopologyURL(inboundRouteURL, m.topology)
// 		if err != nil {
// 			return fmt.Errorf("asTopologyURL(%q): %w", inboundRouteURL, err)
// 		}
// 		hmacSecret := message.GenerateSecret()
// 		publicKey, err := m.secureInbound(protectedURL, hmacSecret, handlerControlTimeout(selfService))
// 		if err != nil {
// 			return fmt.Errorf("secureInbound(%q): %w", protectedRouteURL, err)
// 		}
// 		inOutbounds[inboundURL.String()] = RouteCredential{
// 			RouteURL:  protectedURL.String(),
// 			PublicKey: publicKey,
// 			Secret:    hmacSecret,
// 		}
// 	}

// 	msg := message.Request{
// 		Command: Handshake,
// 		Parameters: datatype.New().
// 			Set("manager-hmac-secret", secret).
// 			Set("manager-url", managerLink.String()).
// 			Set("in-inbounds", map[string]RouteCredential{}).
// 			Set("in-outbounds", inOutbounds),
// 	}
// 	signature, err := message.Sign(msg.String(), m.curveSecretKey)
// 	if err != nil {
// 		return fmt.Errorf("message.Sign: %w", err)
// 	}
// 	msg.Parameters.Set("signature", signature)

// 	reply, err := node.Request(&msg)
// 	if err != nil {
// 		return fmt.Errorf("socket.Request(%q): %w", Handshake, err)
// 	}
// 	if !reply.IsOK() {
// 		return fmt.Errorf("reply.Message: %s", reply.ErrorMessage())
// 	}

// 	replyOutboundsKV, err := reply.ReplyParameters().NestedValue("outbounds")
// 	if err != nil {
// 		replyOutboundsKV = datatype.New()
// 	}

// 	for _, cred := range inOutbounds {
// 		protectedURL, err := mushroom.Parse(cred.RouteURL)
// 		if err != nil {
// 			return fmt.Errorf("mushroom.Parse(%q): %w", cred.RouteURL, err)
// 		}
// 		depPublicKey, err := replyOutboundsKV.StringValue(cred.RouteURL)
// 		if err != nil {
// 			return fmt.Errorf("reply outbounds public key for %q: %w", cred.RouteURL, err)
// 		}
// 		if err := m.allowPublicKey(cred.RouteURL, protectedURL, depPublicKey); err != nil {
// 			return fmt.Errorf("allowPublicKey(%q): %w", cred.RouteURL, err)
// 		}
// 	}

// 	return nil
// }

// func (m *NodeHandshake) whitelistDepsInDeps(depURL string) error {
// 	depFullURL, err := asTopologyURL(depURL, m.topology)
// 	if err != nil {
// 		return fmt.Errorf("asTopologyURL(%q): %w", depURL, err)
// 	}

// 	depServiceURL := depFullURL.As(mushroom.SERVICE)

// 	secret := m.managerSelf.getOutboundHmacSecret(depServiceURL.AsDereference().String())
// 	if secret == "" {
// 		return fmt.Errorf("dep %q has no self-in-deps handshake`` secret", depURL)
// 	}

// 	depServiceConfig, err := m.topology.Service(depServiceURL.AsDereference().String())
// 	if err != nil {
// 		return fmt.Errorf("topology.Service(%q): %w", depServiceURL.AsDereference().String(), err)
// 	}

// 	depManagerConfig, err := depServiceConfig.HandlerByCategory(config.ServiceManagerCategory)
// 	if err != nil {
// 		return fmt.Errorf("dep %q manager handler: %w", depServiceConfig.Name, err)
// 	}
// 	depManagerAsIndependent, ok := depManagerConfig.AsIndependentHandler()
// 	if !ok {
// 		return fmt.Errorf("dep %q manager handler is invalid", depServiceConfig.Name)
// 	}

// 	socket, err := protocolClient.New(
// 		depManagerAsIndependent.Endpoint.Id,
// 		depManagerAsIndependent.Endpoint.Port,
// 		protocolClient.HandlerType(depManagerAsIndependent.Type),
// 	)
// 	if err != nil {
// 		return fmt.Errorf("client.New: %w", err)
// 	}
// 	defer socket.Close()

// 	node := &topology.Client{Socket: socket}
// 	depManagerLink := depServiceURL.New(config.ServiceManagerCategory)
// 	pubKey, err := getPublicKeyFromConfig(depManagerLink.String(), m.topology)
// 	if err != nil {
// 		return fmt.Errorf("getPublicKeyFromConfig(%q): %w", depManagerLink.String(), err)
// 	}
// 	node.Socket.Allow(pubKey)
// 	node.Socket.Secure(m.curveSecretKey)
// 	node.Timeout(handshakeRequestTimeout(depServiceConfig))
// 	node.Attempt(1)

// 	depInbounds, err := filterTopologyInbounds(m.topologyInbounds, depServiceURL, m.serviceURL, true)
// 	if err != nil {
// 		return fmt.Errorf("filterTopologyInbounds: %w", err)
// 	}

// 	inbounds := make(map[string]string, len(depInbounds))
// 	for route, inboundURL := range depInbounds {
// 		inbounds[route] = inboundURL.String()
// 	}
// 	if len(inbounds) == 0 {
// 		return nil
// 	}

// 	if err := node.Socket.Whitelist(message.Any, secret); err != nil {
// 		return fmt.Errorf("socket.Whitelist(%q): %w", message.Any, err)
// 	}

// 	msg := message.Request{
// 		Command: SecureEdges,
// 		Parameters: datatype.New().
// 			Set(SecureEdgesProgressParam, SecureEdgesProgressDepsInDeps).
// 			Set("inbounds", inbounds),
// 	}

// 	reply, err := node.Request(&msg)
// 	if err != nil {
// 		return fmt.Errorf("socket.Request(%q): %w", SecureEdges, err)
// 	}
// 	if !reply.IsOK() {
// 		return fmt.Errorf("reply.Message: %s", reply.ErrorMessage())
// 	}
// 	return nil
// }

func (m *NodeHandshake) checkStatus(node *topology.Client, contextURL string, depStatus *ServiceStatus) (ServiceStatusMsg, error) {
	node.Attempt(1)
	node.Timeout(handshakeRequestTimeoutByProtocol(depStatus.protocol))
	rep, err := node.Request(&message.Request{
		Command: GetServiceStatus,
		Parameters: datatype.New().
			Set("context-url", contextURL).
			Set("service-url", m.serviceURL.String()).
			Set("dep-status", depStatus.toMsg()),
	})
	if err != nil {
		return ServiceStatusMsg{}, err
	}
	if !rep.IsOK() {
		return ServiceStatusMsg{}, fmt.Errorf("reply.Message: %s", rep.ErrorMessage())
	}

	var status ServiceStatusMsg
	statusKV, err := rep.ReplyParameters().NestedValue("dep-status")
	if err != nil {
		return ServiceStatusMsg{}, fmt.Errorf("reply.ReplyParameters().NestedValue(%q): %w", "dep-status", err)
	}
	err = statusKV.Interface(&status)
	if err != nil {
		return ServiceStatusMsg{}, fmt.Errorf("json.Unmarshal(%q): %w", statusKV.String(), err)
	}

	return status, nil
}

func (m *NodeHandshake) connectManager(contextURL string, depStatus *ServiceStatus, recursive ...bool) error {
	if len(recursive) == 0 {
		depStatus.managerCon.lockGoroutine = true
		defer func() {
			depStatus.managerCon.lockGoroutine = false
		}()
	}

	status := depStatus.managerCon.status.Get()
	if status == CONNECTION_STATUS_NOT_SET || status == CONNECTION_STATUS_ACCESS_ERR {
		err := m.whitelistSelfInDeps(contextURL, depStatus)
		if err != nil {
			if errors.Is(err, message.ErrNoCurveKey) {
				depStatus.managerCon.status.Set(CONNECTION_STATUS_CURVE_ERR)
				return m.connectManager(contextURL, depStatus, true)
			} else {
				if status == CONNECTION_STATUS_NOT_SET {
					depStatus.managerCon.status.Tick()
				} else {
					depStatus.managerCon.status.Set(CONNECTION_STATUS_NOT_SET)
				}
			}
			return nil
		}
		depStatus.managerCon.status.Set(CONNECTION_STATUS_NOT_CHECKED)
		return m.connectManager(contextURL, depStatus, true)
	} else if status == CONNECTION_STATUS_CURVE_ERR {
		if err := m.topology.Reload(); err != nil {
			return fmt.Errorf("topology.Reload: %w", err)
		}

		err := m.allowSelfInDep(depStatus)
		if err != nil {
			if errors.Is(err, message.ErrNoCurveKey) {
				depStatus.managerCon.status.Tick()
				return fmt.Errorf("allowSelfInDep(%q): %w", depStatus.serviceURL.String(), err)
			} else {
				depStatus.managerCon.status.Set(CONNECTION_STATUS_NOT_SET)
				return fmt.Errorf("allowSelfInDep(%q): %w", depStatus.serviceURL.String(), err)
			}
		} else {
			// After setting the service, it needs to set the hmac keys
			depStatus.managerCon.status.Set(CONNECTION_STATUS_NOT_SET)
			return m.connectManager(contextURL, depStatus, true)
		}
	} else {
		running, err := probeServiceRunning(depStatus.managerCon.client, depStatus.serviceURL)
		if err != nil {
			if errors.Is(err, message.ErrNoCurveKey) {
				depStatus.managerCon.status.Set(CONNECTION_STATUS_CURVE_ERR)
				return fmt.Errorf("allowSelfInDep(%q): %w", depStatus.serviceURL.String(), err)
			} else if errors.Is(err, message.ErrAccessDenied) {
				depStatus.managerCon.status.Set(CONNECTION_STATUS_ACCESS_ERR)
				return fmt.Errorf("allowSelfInDep(%q): %w", depStatus.serviceURL.String(), err)
			} else {
				if m.logger != nil {
					m.logger.Error("probeServiceRunning", "contextURL", contextURL, "error", err)
				}
				depStatus.managerCon.status.Set(CONNECTION_STATUS_NOT_SET)
				return fmt.Errorf("probeServiceRunning(%q): %w", depStatus.serviceURL.String(), err)
			}
		}
		if !running {
			if status == CONNECTION_STATUS_TIMEOUT {
				depStatus.managerCon.status.Tick()
			} else {
				depStatus.managerCon.status.Set(CONNECTION_STATUS_NOT_SET)
			}
			return fmt.Errorf("service %q is not running", depStatus.serviceURL.String())
		} else {
			depStatus.managerCon.status.Set(CONNECTION_STATUS_CONNECTED)
		}
	}
	return nil
}

func (m *NodeHandshake) handshakeDep(contextURL string, depStatus *ServiceStatus) {
	// first we ensure the manager is connected
	if err := m.connectManager(contextURL, depStatus); err != nil {
		if m.logger != nil {
			m.logger.Error("connectManager", "contextURL", contextURL, "error", err)
		}
	}
	if depStatus.managerCon.status.Get() == CONNECTION_STATUS_CONNECTED {
		// Just in case if checkStatus is taking longer time
		depStatus.managerCon.lockGoroutine = true
		status, err := m.checkStatus(depStatus.managerCon.client, contextURL, depStatus)
		depStatus.managerCon.lockGoroutine = false
		if err != nil { // do not check for error about connection issue, since connectManager should do it
			if m.logger != nil {
				m.logger.Error("checkStatus", "contextURL", contextURL, "error", err)
			}
		} else {
			if len(depStatus.inbounds) > 0 {
				fmt.Printf("dep has inbound for this service secure it\n")
			}
			if len(depStatus.outbounds) > 0 {
				m.secureOutbounds(contextURL, depStatus, status.Outbounds)
			}
			if len(depStatus.inbounds) == 0 && len(depStatus.outbounds) == 0 {
				fmt.Printf("dep has NO inbound and outbound\n")
			}
		}
	}
}

// Handshake waits for deps to become running and exchanges HMAC secrets.
func (m *NodeHandshake) Handshake() error {
	m.handshakeMu.Lock()
	defer m.handshakeMu.Unlock()

	for contextURL, deps := range m.topologyContexts {
		for _, depStatus := range deps {
			// if context is dependency, then context setter handshakes with it so skip here.
			if !depStatus.isHandshakeable(contextURL) {
				continue
			}

			if !depStatus.managerCon.lockGoroutine {
				go m.handshakeDep(contextURL, depStatus)
			}
		}
	}

	// selfInDepsDone := make([]string, 0, len(depURLs))
	// 1. whitelist, and probe service running

	// 2. whitelist manager in deps
	// managerInDepWhitelisted := make(map[string]bool, len(selfInDepsDone))
	// for _, depURL := range selfInDepsDone {
	// 	wg.Add(1)
	// 	go func(depURL string) {
	// 		defer wg.Done()
	// 		if err := m.whitelistManagerInDeps(depURL); err != nil {
	// 			errCh <- fmt.Errorf("whitelistManagerInDeps(%q): %w", depURL, err)
	// 			return
	// 		}
	// 		mu.Lock()
	// 		managerInDepWhitelisted[depURL] = true
	// 		mu.Unlock()
	// 	}(depURL)
	// }
	// wg.Wait()

	// for _, depURL := range selfInDepsDone {
	// 	if !managerInDepWhitelisted[depURL] {
	// 		continue
	// 	}
	// 	wg.Add(1)
	// 	go func(depURL string) {
	// 		defer wg.Done()
	// 		if err := m.whitelistDepsInDeps(depURL); err != nil {
	// 			errCh <- fmt.Errorf("whitelistDepsInDeps(%q): %w", depURL, err)
	// 		}
	// 	}(depURL)
	// }
	// wg.Wait()
	// close(errCh)

	// for e := range errCh {
	// 	return e
	// }

	m.maybeStartBackgroundHandshake()
	return nil
}
