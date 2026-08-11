package manager

// import (
// 	"errors"
// 	"fmt"
// 	"strings"
// 	"sync"
// 	"time"

// 	"github.com/noPerfection/datatype"
// 	protocolHandler "github.com/noPerfection/protocol/handler"
// 	"github.com/noPerfection/protocol/message"
// 	"github.com/noPerfection/service/mushroom"
// 	"github.com/noPerfection/service/zap"
// 	"github.com/noPerfection/topology"
// 	"github.com/noPerfection/topology/config"
// )

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
	"github.com/noPerfection/topology"
	"github.com/noPerfection/topology/config"
)

const (
	// Progress = 1/4?
	CONNECTION_STATUS_NOT_SET     = "not-set"      // 1. as default, no connection established -> request handshake
	CONNECTION_STATUS_NOT_CHECKED = "not-checked"  // 2. once handshaked, set not-checked -> is-service-running
	CONNECTION_STATUS_ACCESS_ERR  = "access-error" // 3. if hmac error -> request handshake
	CONNECTION_STATUS_CURVE_ERR   = "curve-error"  // 4. if curve mismatch -> request handshake
	CONNECTION_STATUS_CONNECTED   = "connected"    // 5. skip manager
	CONNECTION_STATUS_TIMEOUT     = "timeout"      // timeout -> request is-service-running
)

type ConnectionStatus struct {
	status      string
	updatedTime time.Time
	tick        time.Time
}

func (c *ConnectionStatus) Set(status string) bool {
	if c.status == status {
		c.updatedTime = time.Now()
		c.tick = time.Now()
		return false
	}
	c.status = status
	c.updatedTime = time.Now()
	c.tick = time.Now()
	return true
}

// Returns status, if not set return CONNECTION_STATUS_NOT_SET ('not-set')
func (c *ConnectionStatus) Get() string {
	if c.status == "" {
		return CONNECTION_STATUS_NOT_SET
	}
	return c.status
}

func (c *ConnectionStatus) Tick() {
	c.tick = time.Now()
}

func (c *ConnectionStatus) String() string {
	if c.status == "" {
		return CONNECTION_STATUS_NOT_SET
	}
	return fmt.Sprintf("%s@%s, tick: %s", c.status, c.updatedTime.Format(time.RFC3339), c.tick.Format(time.RFC3339))
}

type ManagerConnection struct {
	status        ConnectionStatus
	client        *topology.Client
	hmac          string
	hash          string
	lockGoroutine bool
}

type InboundConnection struct {
	status ConnectionStatus
	depURL mushroom.TopologyURL
}

type OutboundConnection struct {
	status ConnectionStatus
	depURL mushroom.TopologyURL
}

type ServiceStatus struct {
	serviceURL mushroom.TopologyURL
	// inproc | ipc | tcp
	protocol      string
	managerCon    *ManagerConnection
	inboundCons   map[string]*InboundConnection
	outboundCons  map[string]*OutboundConnection
	inboundEdges  map[string]*InboundConnection
	outboundEdges map[string]*OutboundConnection
	inbounds      map[string]*InboundConnection
	outbounds     map[string]*OutboundConnection
}

type Context map[string]map[string]*ServiceStatus

func (s *ServiceStatus) isHandshakeable() bool {
	return len(s.inbounds) > 0 || len(s.inboundCons) > 0 || len(s.outboundCons) > 0 || len(s.inboundEdges) > 0 || len(s.outboundEdges) > 0
}

func (c *Context) Print() {
	for contextURL, deps := range *c {
		fmt.Printf("        ContextURL: %s, deps: %d\n", contextURL, len(deps))
		for depDeref, depStatus := range deps {
			fmt.Printf("          DepURL: %s, Protocol: %s\n", depDeref, depStatus.protocol)
			fmt.Printf("              ManagerCon: %s\n", depStatus.managerCon.status.String())
			fmt.Printf("              Inbounds:   %v\n", len(depStatus.inbounds) > 0)
			fmt.Printf("              Outbounds:  %v\n", len(depStatus.outbounds) > 0)
			fmt.Printf("              InboundCons:  %v, InboundEdges:  %v\n", len(depStatus.inboundCons) > 0, len(depStatus.inboundEdges) > 0)
			fmt.Printf("              OutboundCons: %v, OutboundEdges: %v\n", len(depStatus.outboundCons) > 0, len(depStatus.outboundEdges) > 0)
		}
	}
}

type ManagerInterface interface {
	routeInboundsTopology() *topology.Client
	routeInboundsServiceDeref() (string, error)
	routeInboundsMushroomURL() (mushroom.TopologyURL, error)
	routeHandlerCommands(category string) ([]string, error)

	isOutboundHmacExist(managerURL string) bool
	// setOutboundHmacSecret(depServiceURL string, secret string)
	getOutboundHmacSecret(depServiceURL string) string

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

// type DepStatus struct {
// 	Status int
// }

// type HandshakeStatus struct {
// 	DepStatus map[string]struct{}
// }

// const NOT_INITIATED = 1
// const WHITELIST_SELF_IN_DEPS = 2
// const ALLOW_SELF_IN_DEP = 3
// const WHITELIST_MANAGER_IN_DEPS = 4
// const WHITELIST_DEPS_IN_DEPS = 5

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
						context[contextURL][depURL.String()].outbounds[route] = &OutboundConnection{
							depURL: outboundURL,
							status: ConnectionStatus{
								status:      CONNECTION_STATUS_NOT_SET,
								updatedTime: time.Now(),
								tick:        time.Now(),
							},
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

// Returns handler dereferences as full paths built from dependencies.
// Dependency URLs may be service or handler links; when no handler category
// is present (e.g. proxy or extension deps), the default category (main) is applied.
//
// Returns the list of dependencies as a handler link dereference.
// Dependencies are all its handler-deps and command-deps.
//
// Example:
//
//	service: hello-world
//	dependencies:
//	  - proxy: entrypoint-proxy.main
//	  - extension: ai.main
//
//	returns:
//	  - *pkg:json/./#noPerfection.json?var=services[name:default-name-proxy]&category=main = {}
//	  - *pkg:json/./#noPerfection.json?var=services[name:ai]&category=main = {}
func getDepDereferences(serviceURL mushroom.TopologyURL, m ManagerInterface, topology topology.TopologyInterface) (map[string]struct{}, error) {
	serviceConfig, err := topology.Service(serviceURL.AsDereference().String())
	if err != nil {
		return nil, fmt.Errorf("topology.Service: %w", err)
	}

	depURLs := make(map[string]struct{})
	addDep := func(u string) error {
		link, err := topology.GetLink(u)
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

func (m *NodeHandshake) onHandshake(req message.RequestInterface) message.ReplyInterface {
	if m.topology == nil {
		return req.Fail("topology is nil")
	}
	managerRawURL, err := req.RouteParameters().StringValue("service-url")
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
	contextRawURL, err := req.RouteParameters().StringValue("context-url")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('context-url'): %v", err))
	}
	if contextRawURL == "" {
		return req.Fail("context-url is required")
	}
	contextURL, err := mushroom.Parse(contextRawURL)
	if err != nil {
		return req.Fail(fmt.Sprintf("mushroom.Parse(%q): %v", contextRawURL, err))
	}
	if !contextURL.IsHandlerExist() {
		return req.Fail("context-url must include a handler category")
	}
	if contextURL.HandlerCategory() != config.ServiceManagerCategory {
		return req.Fail(fmt.Sprintf("context-url handler category must be %q", config.ServiceManagerCategory))
	}
	if contextURL.Equal(managerURL, mushroom.SERVICE) {
		fmt.Println("context and service are same ", m.serviceURL.String(), "from", managerURL.String())
	} else {
		fmt.Println("context and service are different")
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

// func (m *NodeHandshake) onSecureOutbounds(req message.RequestInterface) message.ReplyInterface {
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

// 	outboundsRaw, err := req.RouteParameters().NestedValue("in-outbounds")
// 	if err != nil {
// 		return req.Fail(fmt.Sprintf("req.RouteParameters().NestedValue('in-outbounds'): %v", err))
// 	}
// 	outbounds := make(map[string]RouteCredential)
// 	if err := outboundsRaw.Interface(&outbounds); err != nil {
// 		return req.Fail(fmt.Sprintf("outboundsRaw.Interface: %v", err))
// 	}

// 	if m.managerSelf.isOutboundHmacExist(managerURL.String()) {
// 		return req.Fail("manager-url conflicts with outbound secrets")
// 	}

// 	selfService, err := m.managerSelf.selfService()
// 	if err != nil {
// 		return req.Fail(fmt.Sprintf("selfService: %v", err))
// 	}

// 	replyOutbounds := make(map[string]string, len(outbounds))
// 	for depRoute, cred := range outbounds {
// 		if cred.RouteURL == "" {
// 			continue
// 		}
// 		routeURL, err := mushroom.Parse(depRoute)
// 		if err != nil {
// 			return req.Fail(fmt.Sprintf("mushroom.Parse(%q): %v", depRoute, err))
// 		}
// 		pubKey, err := m.prepareOutboundContext(routeURL)
// 		if err != nil {
// 			return req.Fail(fmt.Sprintf("prepareOutboundContext(%q): %v", depRoute, err))
// 		}
// 		if err := m.registerOutboundContext(depRoute, cred.RouteURL, cred.Secret, cred.PublicKey); err != nil {
// 			return req.Fail(fmt.Sprintf("registerOutboundContext(%q): %v", depRoute, err))
// 		}
// 		replyOutbounds[cred.RouteURL] = pubKey
// 	}

// 	return req.Ok(datatype.New().Set("inbounds", replyInbounds).Set("outbounds", replyOutbounds))
// }

// func (m *NodeHandshake) onSecureEdges(req message.RequestInterface) message.ReplyInterface {
// 	if m.topology == nil {
// 		return req.Fail("topology is nil")
// 	}

// 	progress, _ := req.RouteParameters().StringValue(SecureEdgesProgressParam)
// 	if progress == "" {
// 		return req.Fail("progress is required")
// 	}

// 	switch progress {
// 	case SecureEdgesProgressManagerInDeps:
// 		outboundsRaw, err := req.RouteParameters().NestedValue("outbounds")
// 		if err != nil {
// 			return req.Fail(fmt.Sprintf("req.RouteParameters().NestedValue('outbounds'): %v", err))
// 		}
// 		outbounds := make(map[string]string)
// 		if err := outboundsRaw.Interface(&outbounds); err != nil {
// 			return req.Fail(fmt.Sprintf("outboundsRaw.Interface: %v", err))
// 		}
// 		for depRoute, depURL := range outbounds {
// 			if depURL == "" {
// 				return req.Fail(fmt.Sprintf("outbound %q has no dep", depRoute))
// 			}
// 			if err := m.allowOutboundManager(depURL); err != nil {
// 				return req.Fail(fmt.Sprintf("allowOutboundManager(%q from %q): %v", depURL, depRoute, err))
// 			}
// 		}
// 		return req.Ok(datatype.New())
// 	case SecureEdgesProgressDepsInDeps:
// 		inboundsRaw, err := req.RouteParameters().NestedValue("inbounds")
// 		if err != nil {
// 			return req.Fail(fmt.Sprintf("req.RouteParameters().NestedValue('inbounds'): %v", err))
// 		}
// 		inbounds := make(map[string]string)
// 		if err := inboundsRaw.Interface(&inbounds); err != nil {
// 			return req.Fail(fmt.Sprintf("inboundsRaw.Interface: %v", err))
// 		}
// 		grouped := make(map[string]map[string]string)
// 		for protectedRouteURL, inboundRouteURL := range inbounds {
// 			inboundURL, err := asTopologyURL(inboundRouteURL, m.topology)
// 			if err != nil {
// 				return req.Fail(fmt.Sprintf("asTopologyURL(%q): %v", inboundRouteURL, err))
// 			}
// 			depURL := inboundURL.As(mushroom.SERVICE).AsDereference().String()
// 			if grouped[depURL] == nil {
// 				grouped[depURL] = make(map[string]string)
// 			}
// 			grouped[depURL][protectedRouteURL] = inboundRouteURL
// 		}
// 		for depURL, callerInbounds := range grouped {
// 			if err := m.handshakeCallerInboundDep(depURL, callerInbounds); err != nil {
// 				return req.Fail(fmt.Sprintf("handshakeCallerInboundDep(%q): %v", depURL, err))
// 			}
// 		}
// 		return req.Ok(datatype.New())
// 	default:
// 		return req.Fail(fmt.Sprintf("unknown secure-edges progress %q", progress))
// 	}
// }

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

// // prepareOutboundContext returns the control outbound public key for outboundURL.
// // The outbound url should be on this service.
// //
// // When the handler control has no outbound identity yet, an ephemeral CURVE key is generated
// // without restarting the handler socket.
// //
// // Returns public key of the handler control outbound identity.
// func (m *NodeHandshake) prepareOutboundContext(outboundURL mushroom.TopologyURL) (string, error) {
// 	if !outboundURL.IsRouteExist() {
// 		return "", fmt.Errorf("outboundURL.IsRouteExist() is false: %q", outboundURL.String())
// 	}
// 	selfServiceURL, err := asTopologyURL(m.serviceURL.String(), m.topology)
// 	if err != nil {
// 		return "", fmt.Errorf("asTopologyURL(self): %w", err)
// 	}
// 	if !outboundURL.Equal(selfServiceURL, mushroom.SERVICE) {
// 		return "", fmt.Errorf("outbound route %q is not on this service", outboundURL.String())
// 	}
// 	handlerCategory := outboundURL.HandlerLink().HandlerCategory()

// 	if handlerCategory == config.ServiceManagerCategory {
// 		publicKey := m.managerSelf.PublicKey()
// 		if publicKey == "" {
// 			return "", fmt.Errorf("manager public key is empty")
// 		}
// 		return publicKey, nil
// 	}

// 	selfService, err := m.managerSelf.selfService()
// 	if err != nil {
// 		return "", fmt.Errorf("selfService: %w", err)
// 	}
// 	publicKey, err := m.managerSelf.requireHandlerSecureOutbound(handlerCategory, handlerControlTimeout(selfService))
// 	if err != nil {
// 		return "", fmt.Errorf("control.SecureOutbound(%q): %w", handlerCategory, err)
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

// // registerOutboundContext registers npac + control outbound access from inboundURL to outboundURL.
// func (m *NodeHandshake) registerOutboundContext(inboundURL, outboundURL, secret, outboundPublicKey string) error {
// 	localURL, err := mushroom.Parse(inboundURL)
// 	if err != nil {
// 		return fmt.Errorf("mushroom.Parse(%q): %w", inboundURL, err)
// 	}
// 	localCmd := localURL.AdditionalProps["command"]
// 	if localCmd == "" {
// 		return fmt.Errorf("route %q has no command", inboundURL)
// 	}

// 	remoteURL, err := mushroom.Parse(outboundURL)
// 	if err != nil {
// 		return fmt.Errorf("mushroom.Parse(%q): %w", outboundURL, err)
// 	}
// 	cmd := remoteURL.AdditionalProps["command"]
// 	if cmd == "" {
// 		return fmt.Errorf("route %q has no command", outboundURL)
// 	}

// 	endpoint, err := endpointForRouteURL(outboundURL, m.topology)
// 	if err != nil {
// 		return fmt.Errorf("endpointForRouteURL(%q): %w", outboundURL, err)
// 	}

// 	autocontext := protocolClient.NewAutocontext()
// 	if autocontext == nil {
// 		return fmt.Errorf("failed to create npac autocontext")
// 	}
// 	defer func() { _ = autocontext.Close() }()

// 	remoteHandlerURL := remoteURL.As(mushroom.HANDLER).String()
// 	if err := autocontext.RegisterOutbound(endpoint, remoteHandlerURL, outboundPublicKey); err != nil {
// 		if !strings.Contains(err.Error(), "already registered") {
// 			return fmt.Errorf("npac.RegisterOutbound(%q): %w", remoteHandlerURL, err)
// 		}
// 	}

// 	handlerCategory := localURL.HandlerLink().HandlerCategory()

// 	selfService, err := m.managerSelf.selfService()
// 	if err != nil {
// 		return fmt.Errorf("selfService: %w", err)
// 	}

// 	var controlTimeout time.Duration
// 	if handlerCategory == config.ServiceManagerCategory {
// 		controlTimeout = managerProbeTimeout(selfService)
// 	} else {
// 		controlTimeout = handlerControlTimeout(selfService)
// 	}
// 	if err := m.managerSelf.registerHandlerOutbounds(handlerCategory, endpoint, outboundPublicKey, cmd, secret, outboundURL, localCmd, controlTimeout); err != nil {
// 		return fmt.Errorf("registerHandlerOutbounds(%q): %w", outboundURL, err)
// 	}

// 	return nil
// }

// func (m *NodeHandshake) allowPublicKey(inboundURL string, routeTopologyURL mushroom.TopologyURL, depPublicKey string) error {
// 	inboundMushroomURL, err := mushroom.Parse(inboundURL)
// 	if err != nil {
// 		return fmt.Errorf("mushroom.Parse(%q): %w", inboundURL, err)
// 	}
// 	zap.AuthCurveAdd(inboundMushroomURL.As(mushroom.HANDLER).String(), depPublicKey, routeTopologyURL.As(mushroom.HANDLER))
// 	return nil
// }

// allowSelfInDep ensures this manager's CURVE public key is listed in dep's
// parameters.allowed so the dep manager handler can authenticate us.
func (m *NodeHandshake) allowSelfInDep(depStatus *ServiceStatus) error {
	if m.topology == nil {
		return fmt.Errorf("topology is nil")
	}
	if m.managerSelf.PublicKey() == "" {
		return fmt.Errorf("manager public key is empty")
	}

	fmt.Printf("> allowSelfInDep(%q)\n", depStatus.serviceURL.String())

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
				fmt.Printf(">> setManagerClient(%q)\n", depStatus.serviceURL.String())
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

	fmt.Printf(">>> setManagerClient(%q): %v\n", depServiceURL.AsDereference().String(), depManagerEndpoint.ClientUrl())

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

// func (m *NodeHandshake) secureOutbounds(depStatus *ServiceStatus) error {
// 	outboundsAreMe, err := filterTopologyOutbounds(m.topologyOutbounds, depServiceURL, m.serviceURL, false)
// 	if err != nil {
// 		return fmt.Errorf("filterTopologyOutbounds: %w", err)
// 	}

// 	inOutbounds := make(map[string]RouteCredential)
// 	for depRouteURL, outboundInMeURL := range outboundsAreMe {
// 		hmacSecret := message.GenerateSecret()
// 		publicKey, err := m.secureInbound(outboundInMeURL, hmacSecret, handlerControlTimeout(selfService))
// 		if err != nil {
// 			return fmt.Errorf("secureInbound(%q): %w", depRouteURL, err)
// 		}
// 		inOutbounds[depRouteURL] = RouteCredential{
// 			RouteURL:  outboundInMeURL.String(),
// 			PublicKey: publicKey,
// 			Secret:    hmacSecret,
// 		}
// 	}

// 	msg := message.Request{
// 		Command: Handshake,
// 		Parameters: datatype.New().
// 			Set("in-outbounds", inOutbounds),
// 	}
// 	fmt.Printf("in-outbounds: %v\n", inOutbounds)

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

// 	fmt.Printf("reply outbounds: depRouteURL -> Credentials %v\n", replyOutboundsKV)

// 	for depRouteURL, cred := range inOutbounds {
// 		depRouteTopologyURL, err := mushroom.Parse(depRouteURL)
// 		if err != nil {
// 			return fmt.Errorf("mushroom.Parse(%q): %w", depRouteURL, err)
// 		}
// 		depPublicKey, err := replyOutboundsKV.StringValue(cred.RouteURL)
// 		if err != nil {
// 			return fmt.Errorf("reply outbounds public key for %q: %w", cred.RouteURL, err)
// 		}
// 		fmt.Printf("allowPublicKey for outbound is me: me: %q -> dep: %q\n", cred.RouteURL, depRouteTopologyURL.String())
// 		if err := m.allowPublicKey(cred.RouteURL, depRouteTopologyURL, depPublicKey); err != nil {
// 			return fmt.Errorf("allowPublicKey(%q): %w", depRouteURL, err)
// 		}
// 	}

// 	return nil
// }

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

// 	return nil
// }

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

// // Returns handler dereferences as full paths built from dependencies.
// // Dependency URLs may be service or handler links; when no handler category
// // is present (e.g. proxy or extension deps), the default category (main) is applied.
// //
// // Returns the list of dependencies as a handler link dereference.
// // Dependencies are all its handler-deps and command-deps.
// //
// // Example:
// //
// //	service: hello-world
// //	dependencies:
// //	  - proxy: entrypoint-proxy.main
// //	  - extension: ai.main
// //
// //	returns:
// //	  - *pkg:json/./#noPerfection.json?var=services[name:default-name-proxy]&category=main = {}
// //	  - *pkg:json/./#noPerfection.json?var=services[name:ai]&category=main = {}
// func (m *NodeHandshake) getDepDereferences() (map[string]struct{}, error) {
// 	serviceConfig, err := m.topology.Service(m.serviceURL.AsDereference().String())
// 	if err != nil {
// 		return nil, fmt.Errorf("topology.Service: %w", err)
// 	}

// 	depURLs := make(map[string]struct{})
// 	addDep := func(u string) error {
// 		link, err := m.topology.GetLink(u)
// 		if err != nil {
// 			return fmt.Errorf("topology.GetLink('%s'): %w", u, err)
// 		}
// 		mushroomURL, err := mushroom.Parse(link)
// 		if err != nil {
// 			return fmt.Errorf("mushroom.Parse(%q): %w", link, err)
// 		}
// 		depURLs[mushroomURL.HandlerLink().AsDereference().String()] = struct{}{}
// 		return nil
// 	}

// 	for _, hdep := range serviceConfig.HandlerDeps {
// 		for _, u := range hdep.Proxies {
// 			if err := addDep(u); err != nil {
// 				return nil, err
// 			}
// 		}
// 		for _, u := range hdep.Extensions {
// 			if err := addDep(u); err != nil {
// 				return nil, err
// 			}
// 		}
// 	}
// 	for _, variant := range serviceConfig.Handlers {
// 		h, ok := variant.AsIndependentHandler()
// 		if !ok {
// 			continue
// 		}
// 		for _, cdep := range h.CommandDeps {
// 			for _, u := range cdep.Proxies {
// 				if err := addDep(u); err != nil {
// 					return nil, err
// 				}
// 			}
// 			for _, u := range cdep.Extensions {
// 				if err := addDep(u); err != nil {
// 					return nil, err
// 				}
// 			}
// 		}
// 	}

// 	return depURLs, nil
// }

// func (m *NodeHandshake) whitelistManagerInDeps(depURL string) error {
// 	depFullURL, err := asTopologyURL(depURL, m.topology)
// 	if err != nil {
// 		return fmt.Errorf("asTopologyURL(%q): %w", depURL, err)
// 	}

// 	depServiceURL := depFullURL.As(mushroom.SERVICE)

// 	secret := m.managerSelf.getOutboundHmacSecret(depServiceURL.AsDereference().String())
// 	if secret == "" {
// 		return fmt.Errorf("dep %q has no self-in-deps handshake secret", depURL)
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

// 	depOutbounds, err := filterTopologyOutbounds(m.topologyOutbounds, depServiceURL, m.serviceURL, true)
// 	if err != nil {
// 		return fmt.Errorf("filterTopologyOutbounds: %w", err)
// 	}

// 	outbounds := make(map[string]string, len(depOutbounds))
// 	for route, outboundURL := range depOutbounds {
// 		dep := outboundURL.As(mushroom.SERVICE).AsDereference().String()
// 		depManagerLink := outboundURL.As(mushroom.SERVICE).New(config.ServiceManagerCategory)
// 		if _, err := getPublicKeyFromConfig(depManagerLink.String(), m.topology); err != nil {
// 			continue
// 		}
// 		outbounds[route] = dep
// 	}
// 	if len(outbounds) == 0 {
// 		return nil
// 	}

// 	if err := node.Socket.Whitelist(message.Any, secret); err != nil {
// 		return fmt.Errorf("socket.Whitelist(%q): %w", message.Any, err)
// 	}

// 	msg := message.Request{
// 		Command: SecureEdges,
// 		Parameters: datatype.New().
// 			Set(SecureEdgesProgressParam, SecureEdgesProgressManagerInDeps).
// 			Set("outbounds", outbounds),
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

func (m *NodeHandshake) connectManager(contextURL string, depStatus *ServiceStatus) error {
	depStatus.managerCon.lockGoroutine = true
	defer func() {
		depStatus.managerCon.lockGoroutine = false
	}()
	status := depStatus.managerCon.status.Get()
	if status == CONNECTION_STATUS_NOT_SET || status == CONNECTION_STATUS_ACCESS_ERR {
		err := m.whitelistSelfInDeps(contextURL, depStatus)
		if err != nil {
			if errors.Is(err, message.ErrAccessDenied) {
				depStatus.managerCon.status.Set(CONNECTION_STATUS_ACCESS_ERR)
				return fmt.Errorf("whitelistSelfInDeps(%q): %w", depStatus.serviceURL.String(), err)
			} else if !errors.Is(err, message.ErrReqTimeout) {
				if status == CONNECTION_STATUS_NOT_SET {
					depStatus.managerCon.status.Tick()
				} else {
					depStatus.managerCon.status.Set(CONNECTION_STATUS_NOT_SET)
				}
			} else {
				depStatus.managerCon.status.Set(CONNECTION_STATUS_TIMEOUT)
			}
			return nil
		}
		depStatus.managerCon.status.Set(CONNECTION_STATUS_NOT_CHECKED)
		return nil
	} else if status == CONNECTION_STATUS_CURVE_ERR {
		fmt.Printf("3. allowSelfInDep(%s)\n", depStatus.serviceURL.String())

		if err := m.topology.Reload(); err != nil {
			return fmt.Errorf("topology.Reload: %w", err)
		}

		err := m.allowSelfInDep(depStatus)
		if err != nil {
			if errors.Is(err, message.ErrNoCurveKey) {
				depStatus.managerCon.status.Tick()
				return fmt.Errorf("allowSelfInDep(%q): %w", depStatus.serviceURL.String(), err)
			} else {
				fmt.Printf("allowSelfInDep(%q): %v\n", depStatus.serviceURL.String(), err)
				depStatus.managerCon.status.Set(CONNECTION_STATUS_NOT_SET)
				return m.connectManager(contextURL, depStatus)
			}
		} else {
			// After setting the service, it needs to set the hmac keys
			depStatus.managerCon.status.Set(CONNECTION_STATUS_NOT_SET)
			return m.connectManager(contextURL, depStatus)
		}
	} else {
		running, err := probeServiceRunning(depStatus.managerCon.client, depStatus.serviceURL)
		fmt.Printf("probeServiceRunning(%q): %t, %v for %s\n", depStatus.serviceURL.String(), running, err, status)
		if err != nil {
			if errors.Is(err, message.ErrNoCurveKey) {
				fmt.Printf("allowSelfInDep(%q): set-to-no-curve\n", depStatus.serviceURL.String())
				depStatus.managerCon.status.Set(CONNECTION_STATUS_CURVE_ERR)
				return nil
			} else if errors.Is(err, message.ErrAccessDenied) {
				fmt.Printf("allowSelfInDep(%q): set-to-access-err\n", depStatus.serviceURL.String())
				depStatus.managerCon.status.Set(CONNECTION_STATUS_ACCESS_ERR)
				return nil
			} else {
				if m.logger != nil {
					m.logger.Error("probeServiceRunning", "contextURL", contextURL, "error", err)
				} else {
					fmt.Printf("probeServiceRunning(%q): undefined as %v\n", depStatus.serviceURL.String(), err)
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
	fmt.Printf("> handshakeDep: locked? %v, status: %s\n", depStatus.managerCon.lockGoroutine, depStatus.managerCon.status.String())
	fmt.Printf(">    connectManager\n")
	// first we ensure the manager is connected
	err := m.connectManager(contextURL, depStatus)
	fmt.Printf("<    connectManager: %v status: %s\n", err, depStatus.managerCon.status.String())
	if err != nil {
		if m.logger != nil {
			m.logger.Error("connectManager", "contextURL", contextURL, "error", err)
		} else {
			fmt.Printf("connectManager(%q): %v\n", contextURL, err)
		}
		return
	}

	if depStatus.managerCon.status.Get() == CONNECTION_STATUS_CONNECTED {
	} else {
		fmt.Printf("< handshakeDep: %s\n", depStatus.managerCon.status.String())
	}
}

// Handshake waits for deps to become running and exchanges HMAC secrets.
func (m *NodeHandshake) Handshake() error {
	m.handshakeMu.Lock()
	defer m.handshakeMu.Unlock()

	fmt.Printf("> Handshake %s\n", m.serviceURL.String())

	// depURLs, err := m.getDepDereferences()
	// if err != nil {
	// 	return fmt.Errorf("getDepURLs: %w", err)
	// }
	// if len(depURLs) == 0 {
	// 	m.maybeStartBackgroundHandshake()
	// 	return nil
	// }

	// const attempts = 2

	for contextURL, deps := range m.topologyContexts {
		for _, depStatus := range deps {
			if !depStatus.isHandshakeable() && contextURL != m.serviceURL.String() {
				fmt.Println("Not handshakeable")
				continue
			}

			if !depStatus.managerCon.lockGoroutine {
				go m.handshakeDep(contextURL, depStatus)
			}
		}
	}
	// var wg sync.WaitGroup
	// var mu sync.Mutex
	// selfInDepsDone := make([]string, 0, len(depURLs))
	// errCh := make(chan error, len(depURLs)*2)
	// for url := range depURLs {
	// 	wg.Add(1)
	// 	go func(depURL string) {
	// 		defer wg.Done()
	// 		start := time.Now()
	// 		fmt.Printf("> is %s service running, attempts %d, time: %s\n", depURL, attempts, start)
	// 		running, runErr := m.managerSelf.IsServiceRunning(depURL, attempts)
	// 		handshaked := false
	// 		fmt.Printf("< is %s service running? %t, err: %v, time: %s\n", depURL, running, runErr, time.Since(start))
	// 		if runErr != nil {
	// 			if errors.Is(runErr, message.ErrAccessDenied) {
	// 				if err := m.whitelistSelfInDeps(depURL); err != nil {
	// 					errCh <- fmt.Errorf("whitelistSelfInDeps(%q): %w", depURL, err)
	// 					return
	// 				}
	// 				handshaked = true
	// 				running, runErr = m.managerSelf.IsServiceRunning(depURL, 1)
	// 			} else if errors.Is(runErr, message.ErrNoCurveKey) {
	// 				if err := m.allowSelfInDep(depURL); err != nil {
	// 					errCh <- fmt.Errorf("allowSelfInDep(%q): %w", depURL, err)
	// 					return
	// 				}
	// 				running, runErr = m.managerSelf.IsServiceRunning(depURL, 1)
	// 				if runErr != nil {
	// 					if err := m.whitelistSelfInDeps(depURL); err != nil {
	// 						errCh <- fmt.Errorf("whitelistSelfInDeps(%q): %w", depURL, err)
	// 						return
	// 					}
	// 					handshaked = true
	// 					running, runErr = m.managerSelf.IsServiceRunning(depURL, 1)
	// 				}
	// 			}
	// 			if runErr != nil {
	// 				errCh <- fmt.Errorf("IsServiceRunning(%q, attempts: %d): %w", depURL, attempts, runErr)
	// 				return
	// 			}
	// 		}
	// 		if !running {
	// 			errCh <- fmt.Errorf("service %q not running, attempts: %d", depURL, attempts)
	// 			return
	// 		}
	// 		if handshaked {
	// 			mu.Lock()
	// 			selfInDepsDone = append(selfInDepsDone, depURL)
	// 			mu.Unlock()
	// 		}
	// 	}(url)
	// }
	// wg.Wait()

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
