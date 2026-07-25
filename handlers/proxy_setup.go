package handlers

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/log"
	protocolClient "github.com/noPerfection/protocol/client"
	protocolHandler "github.com/noPerfection/protocol/handler"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service/mushroom"
	"github.com/noPerfection/topology"
	topologyConfig "github.com/noPerfection/topology/config"
)

const (
	ProxyHandlersCategory        = "_proxy_manager_noperf"
	SetProxyHandlerCommand       = "set-proxy-handler-command"
	IsProxyHandlerExistCommand   = "is-proxy-handler-exist-command"
	IsProxyHandlerRunningCommand = "is-proxy-handler-running-command"
	StartProxyHandlerCommand     = "start-proxy-handler-command"
	StopProxyHandlerCommand      = "stop-proxy-handler-command"
	StartProxyHandlersCommand    = "start-proxy-handlers-command"
	StopProxyHandlersCommand     = "stop-proxy-handlers-command"
	RemoveProxyHandlerCommand    = "remove-proxy-handler-command"
	RequireWhitelistCommand      = "require-whitelist-command"
	CommandsCommand              = "commands-command"
	RequireSecureHandlerCommand  = "require-secure-handler-command"
	SecureOutboundHandlerCommand = "secure-outbound-handler-command"
	AllowHandlerCommand          = "allow-handler-command"
	RequireInboundWhitelistCommand = "require-inbound-whitelist-command"
	RegisterHandlerOutboundsCommand = "register-handler-outbounds-command"

	proxyBroadcastListenTimeout = 5 * time.Minute
	proxyReceiveAttempt         = uint8(0)
)

// Proxy services work with a special type of requests and replies.
// And handles them in a special way: ProxyHandleFunc
// They are all following the message.RequestInterface and message.ReplyInterface interfaces.
// Where is ProxyHandleFunc is concrete case of noPerfection/protocol/handler.HandleFunc
type ProxyRequest struct {
	message.Request
	proxifiedHandler string
	outboundURL      string
	manager          *ProxySetup
}
type ProxyReply struct {
	message.Reply
}

type ProxyHandleFunc func(req ProxyRequest) ProxyReply

type Category string

type ProxifiedHandler struct {
	handler           protocolHandler.Interface
	outboundClients   map[string]outboundClient
	outboundWhitelist map[string]outboundWhitelistEntry
	routes            map[string]ProxyHandleFunc // command => handleFunc; user can do whatever he wants
	proxyConfig       topologyConfig.ProxyHandler // handler's information
	running           bool
	wasStarted        bool
}

type outboundWhitelistEntry struct {
	secret  string
	command string
}

type outboundClient interface {
	Close() error
	Allow(handlerPublicKey string)
	Whitelist(cmd string, secrets ...string) error
}

type syncReplierOutbound struct{ *protocolClient.SyncReplierClient }

func (c *syncReplierOutbound) Allow(handlerPublicKey string) {
	c.SyncReplierClient.Allow(handlerPublicKey)
}

type replierOutbound struct{ *protocolClient.ReplierClient }

func (c *replierOutbound) Allow(handlerPublicKey string) {
	c.ReplierClient.Allow(handlerPublicKey)
}

type publisherOutbound struct{ *protocolClient.PublisherClient }

func (c *publisherOutbound) Allow(handlerPublicKey string) {
	c.PublisherClient.Allow(handlerPublicKey)
}

type pairOutbound struct{ *protocolClient.PairClient }

func (c *pairOutbound) Allow(handlerPublicKey string) {
	c.PairClient.Allow(handlerPublicKey)
}

type workerOutbound struct{ *protocolClient.WorkerClient }

func (c *workerOutbound) Allow(handlerPublicKey string) {
	c.WorkerClient.Allow(handlerPublicKey)
}

type outboundReceiveOptions interface {
	Timeout(time.Duration)
	Attempt(uint8)
}

// ProxySetup owns the proxy handler registry and lifecycle.
// The proxy is a service, only difference from other types of services
// is using noPerfection/topology/config.ProxyHandler instead noPerfection/topology/config.Handler
type ProxySetup struct {
	protocolHandler.Interface
	serviceName string
	serviceLink string
	handlers    map[Category]*ProxifiedHandler
	routes      map[string]ProxyHandleFunc
	logger      *log.Logger
	running     bool
}

var _ message.ReplyInterface = (*ProxyReply)(nil)
var _ message.RequestInterface = (*ProxyRequest)(nil)
var _ message.Packer = (*ProxySetup)(nil)

// Proxy's Request functions
func (request *ProxyRequest) Forward() (ProxyReply, error) {
	proxified := request.manager.handlers[Category(request.proxifiedHandler)]
	client := proxified.outboundClients[request.outboundURL]
	if err := request.applyOutboundWhitelist(proxified, client); err != nil {
		return ProxyReply{}, err
	}

	switch c := client.(type) {
	case protocolClient.RequestInterface:
		reply, err := c.Request(&request.Request)
		if err != nil {
			return ProxyReply{}, err
		}
		return proxyReplyFromReply(reply)
	case *pairOutbound:
		if err := c.Send(&request.Request); err != nil {
			return ProxyReply{}, err
		}
		return ProxyReply{Reply: *request.Ok(datatype.New()).(*message.Reply)}, nil
	case interface {
		protocolClient.SendInterface
		protocolClient.ReceiveInterface
	}:
		if err := c.Send(&request.Request); err != nil {
			return ProxyReply{}, err
		}
		return receiveProxyReply(c.Receive())
	case protocolClient.SendInterface:
		if err := c.Send(&request.Request); err != nil {
			return ProxyReply{}, err
		}
		return ProxyReply{Reply: *request.Ok(datatype.New()).(*message.Reply)}, nil
	case protocolClient.ReceiveInterface:
		return receiveProxyReply(c.Receive())
	default:
		return ProxyReply{}, fmt.Errorf("unsupported outbound client for %q", request.outboundURL)
	}
}

func (request *ProxyRequest) applyOutboundWhitelist(proxified *ProxifiedHandler, client outboundClient) error {
	if proxified == nil || client == nil {
		return nil
	}

	entry, ok := proxified.outboundWhitelistEntry(request.outboundURL, request.CommandName())
	if !ok {
		return nil
	}

	if mushroomURL, err := mushroom.Parse(request.outboundURL); err == nil {
		if publicKey, err := servicePublicKey(mushroomURL); err == nil {
			client.Allow(publicKey)
		}
	}

	if entry.secret == "" {
		return nil
	}

	cmd := request.CommandName()
	if entry.command == cmd || entry.command == "" {
		return client.Whitelist(cmd, entry.secret)
	}
	if entry.command == message.Any {
		return client.Whitelist(message.Any, entry.secret)
	}
	return client.Whitelist(cmd, entry.secret)
}

func (proxified *ProxifiedHandler) outboundWhitelistEntry(outboundURL, command string) (outboundWhitelistEntry, bool) {
	if proxified == nil || proxified.outboundWhitelist == nil {
		return outboundWhitelistEntry{}, false
	}

	if entry, ok := proxified.outboundWhitelist[outboundURL]; ok && entry.secret != "" {
		return entry, true
	}

	for _, entry := range proxified.outboundWhitelist {
		if entry.secret == "" {
			continue
		}
		if entry.command == command || entry.command == message.Any {
			return entry, true
		}
	}

	return outboundWhitelistEntry{}, false
}

func proxyReplyFromReply(reply message.ReplyInterface) (ProxyReply, error) {
	messageReply, ok := reply.(*message.Reply)
	if !ok {
		return ProxyReply{}, fmt.Errorf("outbound reply has unexpected type %T", reply)
	}
	return ProxyReply{Reply: *messageReply}, nil
}

func receiveProxyReply(replies <-chan message.ReplyInterface) (ProxyReply, error) {
	timer := time.NewTimer(proxyBroadcastListenTimeout)
	defer timer.Stop()

	select {
	case reply, ok := <-replies:
		if !ok {
			return ProxyReply{}, fmt.Errorf("outbound receive channel closed")
		}
		return proxyReplyFromReply(reply)
	case <-timer.C:
		return ProxyReply{}, fmt.Errorf("outbound receive timeout")
	}
}

// Proxy's Reply functions
func (reply ProxyReply) IsProxyOk() bool {
	return false
}

// NewProxyHandlers creates an empty proxy handler manager.
func NewProxyHandlers(serviceName string) *ProxySetup {
	if strings.HasPrefix(serviceName, "tmp") {
		panic("serviceName can not start with tmp, since it will turn handler into ipc protocol please change it")
	}
	manager := protocolHandler.NewPair()
	manager.SetEndpoint(message.NewEndpoint(serviceName+ProxyHandlersCategory, 0))

	return &ProxySetup{
		Interface:   manager,
		serviceName: serviceName,
		handlers:    make(map[Category]*ProxifiedHandler),
		routes:      make(map[string]ProxyHandleFunc),
	}
}

/*
This is overwriting any handler's routes to go through the proxy.
*/
func (manager *ProxySetup) handleFunc(request message.RequestInterface) message.ReplyInterface {
	proxyRequest, ok := request.(*ProxyRequest)
	if !ok {
		return request.Fail("proxy request has unexpected type")
	}

	proxified, allowed := manager.proxifiedForCommand(request.CommandName())
	if !allowed {
		return request.Fail("unavailable-route")
	}

	var handleFunc ProxyHandleFunc
	if proxified != nil {
		if err := manager.applyConfiguredForward(proxified, proxyRequest); err != nil {
			return request.Fail(err.Error())
		}
		handleFunc = proxified.routes[request.CommandName()]
		if handleFunc == nil && request.CommandName() != message.Any {
			handleFunc = proxified.routes[message.Any]
		}
	}
	if handleFunc == nil {
		handleFunc = manager.routes[request.CommandName()]
	}
	if handleFunc == nil && request.CommandName() != message.Any {
		handleFunc = manager.routes[message.Any]
	}
	if handleFunc == nil {
		return request.Fail("can not find the proxy handler")
	}

	reply := handleFunc(*proxyRequest)
	return &reply
}

func (manager *ProxySetup) applyConfiguredForward(proxified *ProxifiedHandler, request *ProxyRequest) error {
	if proxified == nil || proxified.proxyConfig.Category == "" {
		return nil
	}

	ref, ok := proxified.proxyConfig.Forward[request.CommandName()]
	if !ok {
		return nil
	}

	resolvedHandler, resolvedURL, err := proxified.resolveConfiguredForward(ref)
	if err != nil {
		return fmt.Errorf("forward outbound %q: %w", ref, err)
	}
	request.proxifiedHandler = resolvedHandler
	request.outboundURL = resolvedURL

	return nil
}

func (proxified *ProxifiedHandler) resolveConfiguredForward(forwardURL string) (string, string, error) {
	for _, outboundURL := range proxified.proxyConfig.Outbounds {
		if outboundURL == forwardURL {
			return proxified.proxyConfig.Category, outboundURL, nil
		}
	}

	return "", "", fmt.Errorf("outbound %q not found", forwardURL)
}

func (manager *ProxySetup) proxifiedForCommand(command string) (*ProxifiedHandler, bool) {
	hasDeniedProxy := false
	for _, proxified := range manager.handlers {
		if proxified.proxyConfig.Category == "" {
			continue
		}
		if proxyConfigAllowsCommand(proxified.proxyConfig, command) {
			return proxified, true
		}
		hasDeniedProxy = true
	}
	if hasDeniedProxy {
		return nil, false
	}

	return nil, true
}

func proxyConfigAllowsCommand(proxyConfig topologyConfig.ProxyHandler, command string) bool {
	if len(proxyConfig.Routes) == 0 {
		return true
	}
	for _, route := range proxyConfig.Routes {
		if route == message.Any || route == command {
			return true
		}
	}
	return false
}

func (manager *ProxySetup) Route(command string, handleFunc ProxyHandleFunc, handlerCategory ...string) error {
	if manager.running {
		return fmt.Errorf("I cant route when its already started. Please stop the handler first or the best way to route before starting the handler")
	}
	if len(handlerCategory) > 1 {
		return fmt.Errorf("too many handler categories")
	}
	if handleFunc == nil {
		return fmt.Errorf("proxy handle function is required when command is '%s'", command)
	}
	if len(handlerCategory) == 0 || handlerCategory[0] == "" {
		if manager.routes == nil {
			manager.routes = make(map[string]ProxyHandleFunc)
		}
		manager.routes[command] = handleFunc
		return nil
	}

	category := Category(handlerCategory[0])
	proxified := manager.handlers[category]
	if proxified == nil {
		proxified = &ProxifiedHandler{
			routes: make(map[string]ProxyHandleFunc),
		}
		manager.handlers[category] = proxified
	}
	proxified.routes[command] = handleFunc

	return nil
}

// SetLogger sets the optional logger for this manager and all registered handlers.
func (manager *ProxySetup) SetLogger(logger *log.Logger) error {
	manager.logger = logger

	if manager.Interface != nil {
		if err := manager.Interface.SetLogger(logger); err != nil {
			return fmt.Errorf("proxy manager SetLogger: %w", err)
		}
	}

	for category, proxified := range manager.handlers {
		if proxified.handler == nil {
			continue
		}
		if err := proxified.handler.SetLogger(logger); err != nil {
			return fmt.Errorf("handler(category: '%s').SetLogger: %w", category, err)
		}
	}

	return nil
}

// Start starts proxy handlers when any are registered.
func (manager *ProxySetup) Start(serviceLink string) error {
	if manager.Interface == nil {
		return fmt.Errorf("proxy manager interface is nil, please create this manager using NewProxyHandlers(serviceName)")
	}
	if manager.running {
		return fmt.Errorf("proxy manager is already started")
	}
	manager.serviceLink = serviceLink
	if err := manager.Interface.Route(SetProxyHandlerCommand, manager.onSetProxyHandler); err != nil {
		return fmt.Errorf("proxy manager Route('%s'): %w", SetProxyHandlerCommand, err)
	}
	if err := manager.Interface.Route(IsProxyHandlerExistCommand, manager.onIsProxyHandlerExist); err != nil {
		return fmt.Errorf("proxy manager Route('%s'): %w", IsProxyHandlerExistCommand, err)
	}
	if err := manager.Interface.Route(IsProxyHandlerRunningCommand, manager.onIsProxyHandlerRunning); err != nil {
		return fmt.Errorf("proxy manager Route('%s'): %w", IsProxyHandlerRunningCommand, err)
	}
	if err := manager.Interface.Route(StartProxyHandlerCommand, manager.onStartProxyHandler); err != nil {
		return fmt.Errorf("proxy manager Route('%s'): %w", StartProxyHandlerCommand, err)
	}
	if err := manager.Interface.Route(StopProxyHandlerCommand, manager.onStopProxyHandler); err != nil {
		return fmt.Errorf("proxy manager Route('%s'): %w", StopProxyHandlerCommand, err)
	}
	if err := manager.Interface.Route(StartProxyHandlersCommand, manager.onStartProxyHandlers); err != nil {
		return fmt.Errorf("proxy manager Route('%s'): %w", StartProxyHandlersCommand, err)
	}
	if err := manager.Interface.Route(StopProxyHandlersCommand, manager.onStopProxyHandlers); err != nil {
		return fmt.Errorf("proxy manager Route('%s'): %w", StopProxyHandlersCommand, err)
	}
	if err := manager.Interface.Route(RemoveProxyHandlerCommand, manager.onRemoveProxyHandler); err != nil {
		return fmt.Errorf("proxy manager Route('%s'): %w", RemoveProxyHandlerCommand, err)
	}
	if err := manager.Interface.Route(RequireWhitelistCommand, manager.onRequireWhitelist); err != nil {
		return fmt.Errorf("proxy manager Route('%s'): %w", RequireWhitelistCommand, err)
	}
	if err := manager.Interface.Route(CommandsCommand, manager.onCommands); err != nil {
		return fmt.Errorf("proxy manager Route('%s'): %w", CommandsCommand, err)
	}
	if err := manager.Interface.Route(RequireSecureHandlerCommand, manager.onRequireSecureHandler); err != nil {
		return fmt.Errorf("proxy manager Route('%s'): %w", RequireSecureHandlerCommand, err)
	}
	if err := manager.Interface.Route(SecureOutboundHandlerCommand, manager.onSecureOutboundHandler); err != nil {
		return fmt.Errorf("proxy manager Route('%s'): %w", SecureOutboundHandlerCommand, err)
	}
	if err := manager.Interface.Route(AllowHandlerCommand, manager.onAllowHandler); err != nil {
		return fmt.Errorf("proxy manager Route('%s'): %w", AllowHandlerCommand, err)
	}
	if err := manager.Interface.Route(RequireInboundWhitelistCommand, manager.onRequireInboundWhitelist); err != nil {
		return fmt.Errorf("proxy manager Route('%s'): %w", RequireInboundWhitelistCommand, err)
	}
	if err := manager.Interface.Route(RegisterHandlerOutboundsCommand, manager.onRegisterHandlerOutbounds); err != nil {
		return fmt.Errorf("proxy manager Route('%s'): %w", RegisterHandlerOutboundsCommand, err)
	}
	mushroomURL, err := mushroom.As(serviceLink, ProxyHandlersCategory)
	if err != nil {
		return fmt.Errorf("handlers.AsHandlerLink('%s'): %w", ProxyHandlersCategory, err)
	}
	manager.Interface.SetMushroomURL(mushroomURL.String())
	if err := manager.Interface.Start(); err != nil {
		return fmt.Errorf("proxy manager Start: %w", err)
	}

	manager.running = true
	return nil
}

// Close closes all registered handlers.
func (manager *ProxySetup) Close() error {
	if manager.Interface == nil {
		return fmt.Errorf("proxy manager interface is nil, please create this manager using NewProxyHandlers(serviceName)")
	}

	for _, proxified := range manager.handlers {
		if proxified == nil {
			continue
		}
		if proxified.running {
			if err := manager.stopProxyHandler(proxified); err != nil {
				return err
			}
			continue
		}
		if err := closeOutboundClients(proxified.outboundClients); err != nil {
			return err
		}
		proxified.outboundClients = nil
		if proxified.handler != nil {
			if err := CloseViaControl(proxified.handler); err != nil {
				return err
			}
		}
	}

	if err := CloseViaControl(manager.Interface); err != nil {
		return err
	}
	manager.running = false

	return nil
}

// Requires 'category' (string) parameter, returns 'commands' ([]string).
func (manager *ProxySetup) onCommands(req message.RequestInterface) message.ReplyInterface {
	category, err := req.RouteParameters().StringValue("category")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('category'): %v", err))
	}

	proxified := manager.handlers[Category(category)]
	if proxified == nil {
		return req.Fail(fmt.Sprintf("handler of %s category is not found", category))
	}
	if proxified.handler == nil {
		return req.Fail(fmt.Sprintf("handler of %s category is nil", category))
	}

	commands := proxified.handler.Commands()
	sort.Strings(commands)
	return req.Ok(datatype.New().Set("commands", commands))
}

func (manager *ProxySetup) proxifiedHandlerControl(category Category) (*protocolClient.Control, error) {
	proxified := manager.handlers[category]
	if proxified == nil || proxified.handler == nil {
		return nil, fmt.Errorf("handler of %s category is not found", category)
	}
	return newHandlerControlClient(proxified.handler)
}

func (manager *ProxySetup) onRequireSecureHandler(req message.RequestInterface) message.ReplyInterface {
	category, err := req.RouteParameters().StringValue("category")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('category'): %v", err))
	}

	controlClient, err := manager.proxifiedHandlerControl(Category(category))
	if err != nil {
		return req.Fail(err.Error())
	}
	defer controlClient.Close()

	controlClient.Attempt(2)
	if timeoutRaw, err := req.RouteParameters().Uint64Value("timeout"); err == nil && timeoutRaw > 0 {
		controlClient.Timeout(time.Duration(timeoutRaw))
	}

	pubKey, err := controlClient.RequireSecure()
	if err != nil {
		return req.Fail(fmt.Sprintf("control.RequireSecure(%q): %v", category, err))
	}

	return req.Ok(datatype.New().Set("public-key", pubKey))
}

func (manager *ProxySetup) onSecureOutboundHandler(req message.RequestInterface) message.ReplyInterface {
	category, err := req.RouteParameters().StringValue("category")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('category'): %v", err))
	}

	controlClient, err := manager.proxifiedHandlerControl(Category(category))
	if err != nil {
		return req.Fail(err.Error())
	}
	defer controlClient.Close()

	pubKey, err := controlClient.SecureOutbound()
	if err != nil {
		return req.Fail(fmt.Sprintf("control.SecureOutbound(%q): %v", category, err))
	}

	return req.Ok(datatype.New().Set("public-key", pubKey))
}

func (manager *ProxySetup) onAllowHandler(req message.RequestInterface) message.ReplyInterface {
	category, err := req.RouteParameters().StringValue("category")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('category'): %v", err))
	}
	publicKey, err := req.RouteParameters().StringValue("public-key")
	if err != nil || publicKey == "" {
		return req.Fail("public-key is required")
	}

	controlClient, err := manager.proxifiedHandlerControl(Category(category))
	if err != nil {
		return req.Fail(err.Error())
	}
	defer controlClient.Close()

	if err := controlClient.Allow(publicKey); err != nil {
		return req.Fail(fmt.Sprintf("control.Allow(%q): %v", category, err))
	}

	return req.Ok(datatype.New())
}

func (manager *ProxySetup) onRequireInboundWhitelist(req message.RequestInterface) message.ReplyInterface {
	category, err := req.RouteParameters().StringValue("category")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('category'): %v", err))
	}
	cmd, err := req.RouteParameters().StringValue("command")
	if err != nil || cmd == "" {
		return req.Fail("command is required")
	}

	controlClient, err := manager.proxifiedHandlerControl(Category(category))
	if err != nil {
		return req.Fail(err.Error())
	}
	defer controlClient.Close()

	secret, err := req.RouteParameters().StringValue("secret")
	if err != nil || secret == "" {
		if err := controlClient.RequireWhitelist(cmd); err != nil {
			return req.Fail(fmt.Sprintf("control.RequireWhitelist(%q): %v", cmd, err))
		}
	} else if err := controlClient.RequireWhitelist(cmd, secret); err != nil {
		return req.Fail(fmt.Sprintf("control.RequireWhitelist(%q): %v", cmd, err))
	}

	return req.Ok(datatype.New())
}

func (manager *ProxySetup) onRegisterHandlerOutbounds(req message.RequestInterface) message.ReplyInterface {
	category, err := req.RouteParameters().StringValue("category")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('category'): %v", err))
	}

	endpointKV, err := req.RouteParameters().NestedValue("endpoint")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().NestedValue('endpoint'): %v", err))
	}
	var endpoint message.Endpoint
	if err := endpointKV.Interface(&endpoint); err != nil {
		return req.Fail(fmt.Sprintf("endpoint param: %v", err))
	}

	commandsKV, err := req.RouteParameters().NestedValue("commands")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().NestedValue('commands'): %v", err))
	}
	var commands map[string]string
	if err := commandsKV.Interface(&commands); err != nil {
		return req.Fail(fmt.Sprintf("commands param: %v", err))
	}

	publicKey, err := req.RouteParameters().StringValue("public-key")
	if err != nil {
		publicKey = ""
	}

	outboundURL, err := req.RouteParameters().StringValue("outbound-url")
	if err != nil || outboundURL == "" {
		return req.Fail("outbound-url is required")
	}
	localCmd, _ := req.RouteParameters().StringValue("local-command")

	remoteMushroomURL, err := mushroom.Parse(outboundURL)
	if err != nil {
		return req.Fail(fmt.Sprintf("mushroom.Parse(%q): %v", outboundURL, err))
	}
	remoteService, err := manager.resolveOutboundService(remoteMushroomURL)
	if err != nil {
		return req.Fail(err.Error())
	}

	proxified := manager.handlers[Category(category)]
	if proxified == nil || proxified.handler == nil {
		return req.Fail(fmt.Sprintf("handler of %s category is not found", category))
	}

	switch remoteService.Type {
	case topologyConfig.ProxyType, topologyConfig.IndependentType:
		if remoteMushroomURL.HandlerLink().HandlerCategory() == topologyConfig.ServiceManagerCategory {
			return req.Fail(fmt.Sprintf("cannot register manager handler %q as outbound on proxy", outboundURL))
		}
		if err := manager.ensureProxifiedOutboundClient(proxified, outboundURL, publicKey); err != nil {
			return req.Fail(fmt.Sprintf("ensureProxifiedOutboundClient(%q): %v", outboundURL, err))
		}

	case topologyConfig.ExtensionType:
		controlClient, err := manager.proxifiedHandlerControl(Category(category))
		if err != nil {
			return req.Fail(err.Error())
		}
		defer controlClient.Close()

		if err := controlClient.RegisterOutbounds(endpoint, publicKey, commands, "", ""); err != nil {
			return req.Fail(fmt.Sprintf("control.RegisterOutbounds(%q): %v", category, err))
		}
		if localCmd != "" {
			if err := npacSecureEdgeCaseOnHandler(proxified.handler, outboundURL, localCmd); err != nil {
				return req.Fail(fmt.Sprintf("NpacSecureEdgeCase(%q, %q): %v", outboundURL, localCmd, err))
			}
		}

	default:
		return req.Fail(fmt.Sprintf("unsupported outbound service type %q for %q", remoteService.Type, outboundURL))
	}

	return req.Ok(datatype.New())
}

func npacSecureEdgeCaseOnHandler(h protocolHandler.Interface, outboundRouteURL, localCmd string) error {
	type secureEdgeCase interface {
		NpacSecureEdgeCase(outbound, cmd string) error
	}
	edgeCase, ok := h.(secureEdgeCase)
	if !ok {
		return fmt.Errorf("handler does not support npac secure edge case")
	}
	if err := edgeCase.NpacSecureEdgeCase(outboundRouteURL, localCmd); err != nil {
		if errors.Is(err, protocolHandler.ErrAlreadyWhitelisted) {
			return nil
		}
		return err
	}
	return nil
}

// Requires 'category' (string) parameter, returns 'exists' (boolean)
func (manager *ProxySetup) onIsProxyHandlerExist(req message.RequestInterface) message.ReplyInterface {
	category, err := req.RouteParameters().StringValue("category")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('category'): %v", err))
	}

	proxified := manager.handlers[Category(category)]
	exists := proxified != nil && proxified.handler != nil

	return req.Ok(datatype.New().Set("exists", exists))
}

// Requires 'category' (string) parameter, returns 'running' (boolean)
func (manager *ProxySetup) onIsProxyHandlerRunning(req message.RequestInterface) message.ReplyInterface {
	category, err := req.RouteParameters().StringValue("category")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('category'): %v", err))
	}

	proxified := manager.handlers[Category(category)]
	if proxified == nil || proxified.handler == nil {
		return req.Fail(fmt.Sprintf("No proxified handler was set, please call %s command to set it first", SetProxyHandlerCommand))
	}

	return req.Ok(datatype.New().Set("running", proxified.running))
}

// Requires 'category' (string) parameter, returns empty reply on success
func (manager *ProxySetup) onStartProxyHandler(req message.RequestInterface) message.ReplyInterface {
	category, err := req.RouteParameters().StringValue("category")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('category'): %v", err))
	}

	proxified := manager.handlers[Category(category)]
	if err := manager.startProxyHandler(proxified); err != nil {
		return req.Fail(err.Error())
	}

	return req.Ok(datatype.New())
}

func (manager *ProxySetup) onStartProxyHandlers(req message.RequestInterface) message.ReplyInterface {
	for category, proxified := range manager.handlers {
		if proxified == nil || proxified.handler == nil || proxified.running {
			continue
		}
		if err := manager.startProxyHandler(proxified); err != nil {
			return req.Fail(fmt.Sprintf("start proxy handler(%s): %v", category, err))
		}
	}

	return req.Ok(datatype.New())
}

// Requires 'category' (string) parameter, returns empty reply on success
func (manager *ProxySetup) onStopProxyHandler(req message.RequestInterface) message.ReplyInterface {
	category, err := req.RouteParameters().StringValue("category")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('category'): %v", err))
	}

	proxified := manager.handlers[Category(category)]
	if err := manager.stopProxyHandler(proxified); err != nil {
		return req.Fail(err.Error())
	}

	return req.Ok(datatype.New())
}

func (manager *ProxySetup) onStopProxyHandlers(req message.RequestInterface) message.ReplyInterface {
	for category, proxified := range manager.handlers {
		if proxified == nil || !proxified.running {
			continue
		}
		if err := manager.stopProxyHandler(proxified); err != nil {
			return req.Fail(fmt.Sprintf("stop proxy handler(%s): %v", category, err))
		}
	}

	return req.Ok(datatype.New())
}

func (manager *ProxySetup) startProxyHandler(proxified *ProxifiedHandler) error {
	if proxified == nil || proxified.handler == nil {
		return fmt.Errorf("No proxified handler was set, please call %s command to set it first", SetProxyHandlerCommand)
	}
	if proxified.running {
		return fmt.Errorf("proxified handler is already running")
	}
	if len(proxified.outboundClients) == 0 {
		outboundClients, err := manager.newOutboundClients(proxified.proxyConfig)
		if err != nil {
			return fmt.Errorf("new outbound clients: %v", err)
		}
		proxified.outboundClients = outboundClients
	}
	startOutboundSubscribers(proxified.outboundClients)
	if proxified.wasStarted {
		controlClient, err := newHandlerControlClient(proxified.handler)
		if err != nil {
			return fmt.Errorf("proxified handler Start: %v", err)
		}
		defer controlClient.Close()
		if _, err = controlClient.StartHandler(); err != nil {
			return fmt.Errorf("proxified handler Start: %v", err)
		}
	} else if err := proxified.handler.Start(); err != nil {
		return fmt.Errorf("proxified handler Start: %v", err)
	} else {
		proxified.wasStarted = true
	}
	proxified.running = true

	return nil
}

func (manager *ProxySetup) stopProxyHandler(proxified *ProxifiedHandler) error {
	if proxified == nil || proxified.handler == nil {
		return fmt.Errorf("No proxified handler was set, please call %s command to set it first", SetProxyHandlerCommand)
	}
	if !proxified.running {
		return fmt.Errorf("proxified handler is not running")
	}
	if err := CloseViaControl(proxified.handler); err != nil {
		return fmt.Errorf("proxified handler Close: %v", err)
	}
	if err := closeOutboundClients(proxified.outboundClients); err != nil {
		return fmt.Errorf("outbound clients Close: %v", err)
	}
	proxified.outboundClients = nil
	proxified.running = false

	return nil
}

// Requires 'category' (string) parameter, returns empty reply on success
func (manager *ProxySetup) onRemoveProxyHandler(req message.RequestInterface) message.ReplyInterface {
	category, err := req.RouteParameters().StringValue("category")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('category'): %v", err))
	}

	proxified := manager.handlers[Category(category)]
	if proxified == nil || proxified.proxyConfig.Category == "" {
		return req.Fail(fmt.Sprintf("No proxified handler was set, please call %s command to set it first", SetProxyHandlerCommand))
	}
	if proxified.running {
		return req.Fail("proxified handler is running, stop it first")
	}

	proxified.handler = nil
	if err := closeOutboundClients(proxified.outboundClients); err != nil {
		return req.Fail(fmt.Sprintf("outbound clients Close: %v", err))
	}
	proxified.outboundClients = nil
	proxified.proxyConfig = topologyConfig.ProxyHandler{}

	return req.Ok(datatype.New())
}

// Requires 'config' (noPerfection/topology/config.ProxyHandler) parameter, returns empty reply on success
func (manager *ProxySetup) onSetProxyHandler(req message.RequestInterface) message.ReplyInterface {
	rawConfig, err := req.RouteParameters().NestedValue("config")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().NestedValue('config'): %v", err))
	}

	var proxyConfig topologyConfig.ProxyHandler
	if err := rawConfig.Interface(&proxyConfig); err != nil {
		return req.Fail(fmt.Sprintf("Can not convert 'config' to noPerfection/topology/config.ProxyHandler: %v", err))
	}
	if err := validateProxyHandlerOutbounds(proxyConfig); err != nil {
		return req.Fail(err.Error())
	}

	category := Category(proxyConfig.Category)
	proxified := manager.handlers[category]
	if proxified == nil {
		proxified = &ProxifiedHandler{
			routes: make(map[string]ProxyHandleFunc),
		}
		manager.handlers[category] = proxified
	} else if proxified.running {
		return req.Fail("not possible to send since the handler is already running, stop")
	}
	if len(proxified.routes) == 0 && len(manager.routes) == 0 {
		return req.Fail(fmt.Sprintf("can not set a proxy since no proxy handle for `%s` or `default` for any command proxy handle is set", category))
	}
	if err := closeOutboundClients(proxified.outboundClients); err != nil {
		return req.Fail(fmt.Sprintf("outbound clients Close: %v", err))
	}
	proxified.outboundClients = nil

	handler, err := newProxyHandler(proxyConfig.Type)
	if err != nil {
		return req.Fail(fmt.Sprintf("newProxyHandler('%s'): %v", proxyConfig.Type, err))
	}
	handler.SetEndpoint(proxyConfig.Endpoint)
	handler.SetPacker(manager)
	if manager.logger != nil {
		if err := handler.SetLogger(manager.logger); err != nil {
			return req.Fail(fmt.Sprintf("handler.SetLogger: %v", err))
		}
	}
	if err = handler.Route(message.Any, manager.handleFunc); err != nil {
		return req.Fail(fmt.Sprintf("Failed to route for proxifying (category: '%s').Route('%s'): %+v", category, message.Any, err))
	}
	mushroomURL, err := mushroom.As(manager.serviceLink, proxyConfig.Category)
	if err != nil {
		return req.Fail(fmt.Sprintf("handlers.AsHandlerLink('%s'): %v", proxyConfig.Category, err))
	}
	handler.SetMushroomURL(mushroomURL.String())

	proxified.handler = handler
	proxified.proxyConfig = proxyConfig
	proxified.wasStarted = false
	proxified.outboundClients, err = manager.newOutboundClients(proxyConfig)
	if err != nil {
		return req.Fail(fmt.Sprintf("new outbound clients: %v", err))
	}
	proxified.running = false

	return req.Ok(datatype.New())
}

// Requires 'outbound-url' (string) and optional 'secret' (string) parameters.
func (manager *ProxySetup) onRequireWhitelist(req message.RequestInterface) message.ReplyInterface {
	outboundURL, err := req.RouteParameters().StringValue("outbound-url")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('outbound-url'): %v", err))
	}
	if outboundURL == "" {
		return req.Fail("outbound-url is required")
	}

	secret, err := req.RouteParameters().StringValue("secret")
	if err != nil {
		secret = ""
	}

	mushroomURL, err := mushroom.Parse(outboundURL)
	if err != nil {
		return req.Fail(fmt.Sprintf("mushroom.Parse(%q): %v", outboundURL, err))
	}

	category := Category(mushroomURL.HandlerCategory())
	proxified := manager.handlers[category]
	if proxified == nil || proxified.handler == nil {
		return req.Fail(fmt.Sprintf("handler of %s category is not found", category))
	}

	cmd := mushroomURL.AdditionalProps["command"]
	if cmd == "" {
		return req.Fail(fmt.Sprintf("outbound-url %q has no command", outboundURL))
	}

	if proxified.outboundWhitelist == nil {
		proxified.outboundWhitelist = make(map[string]outboundWhitelistEntry)
	}
	proxified.outboundWhitelist[outboundURL] = outboundWhitelistEntry{
		secret:  secret,
		command: cmd,
	}

	return req.Ok(datatype.New())
}

func servicePublicKey(mushroomURL mushroom.TopologyURL) (string, error) {
	topologyClient, err := topology.NewClient()
	if err != nil {
		return "", fmt.Errorf("topology.NewClient: %w", err)
	}
	defer topologyClient.Close()

	serviceURL := mushroomURL.As(mushroom.SERVICE).AsDereference().String()
	if link, err := topologyClient.GetLink(serviceURL); err == nil {
		if resolved, err := mushroom.Parse(link); err == nil {
			serviceURL = resolved.As(mushroom.SERVICE).AsDereference().String()
		}
	}

	service, err := topologyClient.Service(serviceURL)
	if err != nil {
		return "", fmt.Errorf("topology.Service(%q): %w", serviceURL, err)
	}
	if service.Parameters == nil {
		return "", fmt.Errorf("service %q has no parameters", service.Name)
	}
	pubKey, ok := service.Parameters["public-key"].(string)
	if !ok || pubKey == "" {
		return "", fmt.Errorf("service %q has no public-key parameter", service.Name)
	}
	return pubKey, nil
}

func validateProxyHandlerOutbounds(proxyConfig topologyConfig.ProxyHandler) error {
	if len(proxyConfig.Outbounds) == 0 {
		return fmt.Errorf("not possible to send since no outbound yet")
	}

	for i, outboundURL := range proxyConfig.Outbounds {
		if outboundURL == "" {
			return fmt.Errorf("outbounds[%d] url is required", i)
		}
	}

	return nil
}

func (manager *ProxySetup) resolveOutboundService(mushroomURL mushroom.TopologyURL) (topologyConfig.Service, error) {
	topologyClient, err := topology.NewClient()
	if err != nil {
		return topologyConfig.Service{}, err
	}
	defer topologyClient.Close()

	serviceURL := mushroomURL.As(mushroom.SERVICE).AsDereference().String()
	if link, err := topologyClient.GetLink(serviceURL); err == nil {
		if resolved, err := mushroom.Parse(link); err == nil {
			serviceURL = resolved.As(mushroom.SERVICE).AsDereference().String()
		}
	}

	service, err := topologyClient.Service(serviceURL)
	if err != nil {
		return topologyConfig.Service{}, fmt.Errorf("topology.Service(%q): %w", serviceURL, err)
	}
	return service, nil
}

func (manager *ProxySetup) ensureProxifiedOutboundClient(proxified *ProxifiedHandler, outboundURL, publicKey string) error {
	if proxified == nil {
		return fmt.Errorf("proxified handler is nil")
	}
	if proxified.outboundClients == nil {
		proxified.outboundClients = make(map[string]outboundClient)
	}
	if client, ok := proxified.outboundClients[outboundURL]; ok && client != nil {
		if publicKey != "" {
			client.Allow(publicKey)
		}
		return nil
	}

	mushroomURL, err := mushroom.Parse(outboundURL)
	if err != nil {
		return fmt.Errorf("mushroom.Parse(%q): %w", outboundURL, err)
	}
	handler, err := manager.resolveOutboundHandler(mushroomURL)
	if err != nil {
		return err
	}
	client, err := newOutboundClient(handler)
	if err != nil {
		return err
	}
	if publicKey != "" {
		client.Allow(publicKey)
	}
	configureOutboundReceiver(client)
	proxified.outboundClients[outboundURL] = client
	if receiver, ok := client.(protocolClient.ReceiveInterface); ok {
		go receiver.Receive()
	}
	return nil
}

func (manager *ProxySetup) resolveOutboundHandler(mushroomURL mushroom.TopologyURL) (topologyConfig.IndependentHandler, error) {
	topologyClient, err := topology.NewClient()
	if err != nil {
		return topologyConfig.IndependentHandler{}, err
	}
	defer topologyClient.Close()

	record, err := topologyClient.Handler(mushroomURL.HandlerLink().AsDereference().String())
	if err != nil {
		return topologyConfig.IndependentHandler{}, err
	}
	handler, ok := record.AsIndependentHandler()
	if !ok {
		return topologyConfig.IndependentHandler{}, fmt.Errorf("outbound %q is not an independent handler", mushroomURL)
	}
	return handler, nil
}

func (manager *ProxySetup) newOutboundClients(proxyConfig topologyConfig.ProxyHandler) (map[string]outboundClient, error) {
	clients := make(map[string]outboundClient)
	for i, outboundURL := range proxyConfig.Outbounds {
		if outboundURL == "" {
			return nil, fmt.Errorf("outbounds[%d] url is required", i)
		}
		mushroomURL, err := mushroom.Parse(outboundURL)
		if err != nil {
			return nil, fmt.Errorf("mushroom.Parse(%q): %w", outboundURL, err)
		}
		handler, err := manager.resolveOutboundHandler(mushroomURL)
		if err != nil {
			_ = closeOutboundClients(clients)
			return nil, fmt.Errorf("outbounds[%d] %q: %w", i, outboundURL, err)
		}
		client, err := newOutboundClient(handler)
		if err != nil {
			_ = closeOutboundClients(clients)
			return nil, fmt.Errorf("outbounds[%d] %q: %w", i, outboundURL, err)
		}
		clients[outboundURL] = client
	}
	return clients, nil
}

func newOutboundClient(handler topologyConfig.IndependentHandler) (outboundClient, error) {
	switch handler.Type {
	case topologyConfig.SyncReplierType:
		client, err := protocolClient.NewSyncReplier(handler.Endpoint.Id, handler.Endpoint.Port)
		if err != nil {
			return nil, err
		}
		return &syncReplierOutbound{client}, nil
	case topologyConfig.ReplierType:
		client, err := protocolClient.NewReplier(handler.Endpoint.Id, handler.Endpoint.Port)
		if err != nil {
			return nil, err
		}
		return &replierOutbound{client}, nil
	case topologyConfig.PublisherType:
		client, err := protocolClient.NewPublisher(handler.Endpoint.Id, handler.Endpoint.Port)
		if err != nil {
			return nil, err
		}
		outbound := &publisherOutbound{client}
		configureOutboundReceiver(outbound)
		return outbound, nil
	case topologyConfig.PairType:
		client, err := protocolClient.NewPair(handler.Endpoint.Id, handler.Endpoint.Port)
		if err != nil {
			return nil, err
		}
		return &pairOutbound{client}, nil
	case topologyConfig.WorkerType:
		client, err := protocolClient.NewWorker(handler.Endpoint.Id, handler.Endpoint.Port)
		if err != nil {
			return nil, err
		}
		return &workerOutbound{client}, nil
	default:
		return nil, fmt.Errorf("unsupported outbound handler type: %s", handler.Type)
	}
}

func configureOutboundReceiver(client outboundClient) {
	options, ok := client.(outboundReceiveOptions)
	if !ok {
		return
	}
	options.Timeout(proxyBroadcastListenTimeout)
	options.Attempt(proxyReceiveAttempt)
}

func closeOutboundClients(clients map[string]outboundClient) error {
	for outboundURL, client := range clients {
		if client == nil {
			continue
		}
		if err := client.Close(); err != nil {
			return fmt.Errorf("outbound client(%s).Close: %w", outboundURL, err)
		}
	}
	return nil
}

func startOutboundSubscribers(clients map[string]outboundClient) {
	for _, client := range clients {
		receiver, ok := client.(protocolClient.ReceiveInterface)
		if !ok {
			continue
		}
		go receiver.Receive()
	}
}

func (manager *ProxySetup) defaultOutbound() (string, string, error) {
	proxified, err := manager.firstProxifiedHandler()
	if err != nil {
		return "", "", err
	}
	if proxified.proxyConfig.Category == "" {
		return "", "", fmt.Errorf("first proxified handler has no proxy config")
	}
	if len(proxified.proxyConfig.Outbounds) == 0 {
		return "", "", fmt.Errorf("first proxified handler has no outbounds")
	}

	return proxified.proxyConfig.Category, proxified.proxyConfig.Outbounds[0], nil
}

func (manager *ProxySetup) resolveOutboundRef(ref string) (string, string, error) {
	if ref == "" {
		return "", "", fmt.Errorf("outbound ref is empty")
	}

	for _, category := range manager.sortedProxifiedCategories() {
		proxified := manager.handlers[Category(category)]
		if proxified == nil {
			continue
		}
		for _, outboundURL := range proxified.proxyConfig.Outbounds {
			if outboundURL == ref {
				return proxified.proxyConfig.Category, outboundURL, nil
			}
		}
	}

	return "", "", fmt.Errorf("outbound %q not found", ref)
}

func (manager *ProxySetup) sortedProxifiedCategories() []string {
	categories := make([]string, 0, len(manager.handlers))
	for category := range manager.handlers {
		categories = append(categories, string(category))
	}
	sort.Strings(categories)
	return categories
}

func (manager *ProxySetup) firstProxifiedHandler() (*ProxifiedHandler, error) {
	if len(manager.handlers) == 0 {
		return nil, fmt.Errorf("no proxified handlers")
	}

	for _, category := range manager.sortedProxifiedCategories() {
		proxified := manager.handlers[Category(category)]
		if proxified != nil && proxified.proxyConfig.Category != "" {
			return proxified, nil
		}
	}

	return nil, fmt.Errorf("no proxified handler configs")
}

func newProxyHandler(handlerType topologyConfig.HandlerType) (protocolHandler.Interface, error) {
	switch handlerType {
	case topologyConfig.SyncReplierType:
		return protocolHandler.NewSyncReplier(), nil
	case topologyConfig.ReplierType:
		return protocolHandler.NewReplier(), nil
	case topologyConfig.PublisherType:
		return protocolHandler.NewPublisher(), nil
	case topologyConfig.PairType:
		return protocolHandler.NewPair(), nil
	case topologyConfig.WorkerType:
		return protocolHandler.NewWorker(), nil
	default:
		return nil, fmt.Errorf("unsupported handler type: %s", handlerType)
	}
}

/****************************************************************************
 * ProxyHandlers also implements the message.Packer interface.
 * Although all messages within noPerfection must follow noPerfection/protocol/message.RequestInterface and noPerfection/protocol/message.ReplyInterface interfaces,
 * With the packers we can add a tail to them and within the structs like this ProxyHandler,
****************************************************************************/

func (manager *ProxySetup) DeserializeRequest(zmqEnvelope []string) (message.RequestInterface, string, error) {
	return manager.deserializeProxyRequest(zmqEnvelope)
}

func (manager *ProxySetup) deserializeProxyRequest(zmqEnvelope []string) (message.RequestInterface, string, error) {
	if err := message.ValidEnvelope(zmqEnvelope); err != nil {
		return nil, "", err
	}

	conId, msg, tail := message.EnvelopeToMessage(zmqEnvelope)
	hmacHash, outboundRef, err := manager.parseProxyRequestTail(tail)
	if err != nil {
		return nil, "", err
	}

	data, err := datatype.NewFromString(msg)
	if err != nil {
		return nil, "", fmt.Errorf("failed to convert message string %s to key-value: %v", msg, err)
	}

	var request message.Request
	err = data.Interface(&request)
	if err != nil {
		return nil, "", fmt.Errorf("failed to convert key-value %v to intermediate interface: %v", data, err)
	}

	if request.String() == "" {
		return nil, "", fmt.Errorf("failed to validate")
	}
	request.SetConId(conId)

	var proxifiedHandler string
	var outboundURL string
	if outboundRef != "" {
		proxifiedHandler, outboundURL, err = manager.resolveOutboundRef(outboundRef)
		if err != nil {
			return nil, "", err
		}
	} else {
		proxifiedHandler, outboundURL, err = manager.defaultOutbound()
		if err != nil {
			return nil, "", err
		}
	}

	return &ProxyRequest{
		Request:          request,
		proxifiedHandler: proxifiedHandler,
		outboundURL:      outboundURL,
		manager:          manager,
	}, hmacHash, nil
}

func (manager *ProxySetup) parseProxyRequestTail(tail []string) (hmacHash string, outboundRef string, err error) {
	switch len(tail) {
	case 0:
		return "", "", nil
	case 1:
		if _, _, resolveErr := manager.resolveOutboundRef(tail[0]); resolveErr == nil {
			return "", tail[0], nil
		}
		return tail[0], "", nil
	default:
		return tail[0], tail[1], nil
	}
}

func proxyEnvelopeHMACTail(hmac ...string) []string {
	if len(hmac) > 0 && hmac[0] != "" {
		return []string{hmac[0]}
	}
	return nil
}

func (manager *ProxySetup) DeserializeReply(zmqEnvelope []string) (message.ReplyInterface, string, error) {
	if err := message.ValidEnvelope(zmqEnvelope); err != nil {
		return nil, "", err
	}

	conId, msg, tail := message.EnvelopeToMessage(zmqEnvelope)
	hmacHash, _ := proxyHMACFromTail(tail)
	data, err := datatype.NewFromString(msg)
	if err != nil {
		return nil, "", fmt.Errorf("datatype.NewFromString: %w", err)
	}

	var reply message.Reply
	err = data.Interface(&reply)
	if err != nil {
		return nil, "", fmt.Errorf("failed to serialize key-value to msg.Reply: %v", err)
	}
	reply.SetConId(conId)

	if reply.String() == "" {
		return nil, "", fmt.Errorf("validation failed")
	}

	return &ProxyReply{Reply: reply}, hmacHash, nil
}

func proxyHMACFromTail(tail []string) (hmacHash string, rest []string) {
	if len(tail) == 0 {
		return "", tail
	}
	return tail[0], tail[1:]
}

func (manager *ProxySetup) SerializeRequest(request message.RequestInterface, hmac ...string) ([]string, error) {
	str := request.String()
	if str == "" {
		return nil, fmt.Errorf("request.String returned an empty string")
	}

	tail := proxyEnvelopeHMACTail(hmac...)
	if proxyRequest, ok := request.(*ProxyRequest); ok {
		if proxyRequest.outboundURL != "" {
			tail = append(tail, proxyRequest.outboundURL)
		}
	}

	return message.MessageToEnvelope(request.ConId(), str, tail...), nil
}

func (manager *ProxySetup) SerializeReply(reply message.ReplyInterface, hmac ...string) ([]string, error) {
	str := reply.String()
	if str == "" {
		return nil, fmt.Errorf("request.String returned an empty string")
	}

	return message.MessageToEnvelope(reply.ConId(), str, proxyEnvelopeHMACTail(hmac...)...), nil
}

func (manager *ProxySetup) EmptyRequest() message.RequestInterface {
	return &ProxyRequest{}
}

func (manager *ProxySetup) EmptyReply() message.ReplyInterface {
	return &ProxyReply{}
}
