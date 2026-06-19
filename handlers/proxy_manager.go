package handlers

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ahmetson/mushroom"
	"github.com/noPerfection/datatype"
	"github.com/noPerfection/log"
	protocolClient "github.com/noPerfection/protocol/client"
	clientPair "github.com/noPerfection/protocol/client/pair"
	clientPublisher "github.com/noPerfection/protocol/client/publisher"
	clientReplier "github.com/noPerfection/protocol/client/replier"
	clientSyncReplier "github.com/noPerfection/protocol/client/sync_replier"
	clientWorker "github.com/noPerfection/protocol/client/worker"
	"github.com/noPerfection/protocol/handler/base"
	handlerConfig "github.com/noPerfection/protocol/handler/config"
	"github.com/noPerfection/protocol/handler/pair"
	"github.com/noPerfection/protocol/handler/publisher"
	"github.com/noPerfection/protocol/handler/replier"
	"github.com/noPerfection/protocol/handler/sync_replier"
	"github.com/noPerfection/protocol/handler/worker"
	"github.com/noPerfection/protocol/message"
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

	proxyBroadcastListenTimeout = 5 * time.Minute
	proxyReceiveAttempt         = uint8(0)
)

// Proxy services work with a special type of requests and replies.
// And handles them in a special way: ProxyHandleFunc
// They are all following the message.RequestInterface and message.ReplyInterface interfaces.
// Where is ProxyHandleFunc is concrete case of noPerfection/protocol/handler/base.HandleFunc
type ProxyRequest struct {
	message.Request
	proxifiedHandler string
	outboundURL      string
	manager          *ProxyHandlers
}
type ProxyReply struct {
	message.Reply
}

type ProxyHandleFunc func(req ProxyRequest) ProxyReply

type Category string

type ProxifiedHandler struct {
	handler         base.Interface
	outboundClients map[string]outboundClient
	routes          map[string]ProxyHandleFunc  // command => handleFunc; user can do whatever he wants
	proxyConfig     topologyConfig.ProxyHandler // handler's information
	running         bool
}

type outboundClient interface {
	Close() error
}

type outboundReceiveOptions interface {
	Timeout(time.Duration)
	Attempt(uint8)
}

// ProxyHandlers owns the proxy handler registry and lifecycle.
// The proxy is a service, only difference from other types of services
// is using noPerfection/topology/config.ProxyHandler instead noPerfection/topology/config.Handler
type ProxyHandlers struct {
	base.Interface
	proxifiedHandlers map[Category]*ProxifiedHandler
	routes            map[string]ProxyHandleFunc
	logger            *log.Logger
	running           bool
}

var _ message.ReplyInterface = (*ProxyReply)(nil)
var _ message.RequestInterface = (*ProxyRequest)(nil)
var _ message.Packer = (*ProxyHandlers)(nil)

// Proxy's Request functions
func (request *ProxyRequest) Forward() (ProxyReply, error) {
	proxified := request.manager.proxifiedHandlers[Category(request.proxifiedHandler)]
	client := proxified.outboundClients[request.outboundURL]

	switch c := client.(type) {
	case protocolClient.RequestInterface:
		reply, err := c.Request(&request.Request)
		if err != nil {
			return ProxyReply{}, err
		}
		return proxyReplyFromReply(reply)
	case *clientPair.Client:
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
func NewProxyHandlers(serviceName string) *ProxyHandlers {
	if strings.HasPrefix(serviceName, "tmp") {
		panic("serviceName can not start with tmp, since it will turn handler into ipc protocol please change it")
	}
	manager := sync_replier.New()
	manager.SetConfig(handlerConfig.New(
		handlerConfig.SyncReplierType,
		serviceName+ProxyHandlersCategory,
		ProxyHandlersCategory,
		0,
	))

	return &ProxyHandlers{
		Interface:         manager,
		proxifiedHandlers: make(map[Category]*ProxifiedHandler),
		routes:            make(map[string]ProxyHandleFunc),
	}
}

/*
This is overwriting any handler's routes to go through the proxy.
*/
func (manager *ProxyHandlers) handleFunc(request message.RequestInterface) message.ReplyInterface {
	proxyRequest, ok := request.(*ProxyRequest)
	if !ok {
		return request.Fail("proxy request has unexpected type")
	}

	proxified, allowed := manager.proxifiedForCommand(request.CommandName())
	if !allowed {
		return request.Fail("access-denied")
	}

	var handleFunc ProxyHandleFunc
	if proxified != nil {
		if err := manager.applyConfiguredForward(proxified, proxyRequest); err != nil {
			return request.Fail(err.Error())
		}
		handleFunc = proxified.routes[request.CommandName()]
		if handleFunc == nil && request.CommandName() != base.Any {
			handleFunc = proxified.routes[base.Any]
		}
	}
	if handleFunc == nil {
		handleFunc = manager.routes[request.CommandName()]
	}
	if handleFunc == nil && request.CommandName() != base.Any {
		handleFunc = manager.routes[base.Any]
	}
	if handleFunc == nil {
		return request.Fail("can not find the proxy handler")
	}

	reply := handleFunc(*proxyRequest)
	return &reply
}

func (manager *ProxyHandlers) applyConfiguredForward(proxified *ProxifiedHandler, request *ProxyRequest) error {
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

func (manager *ProxyHandlers) proxifiedForCommand(command string) (*ProxifiedHandler, bool) {
	hasDeniedProxy := false
	for _, proxified := range manager.proxifiedHandlers {
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
		if route == base.Any || route == command {
			return true
		}
	}
	return false
}

func (manager *ProxyHandlers) Route(command string, handleFunc ProxyHandleFunc, handlerCategory ...string) error {
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
	proxified := manager.proxifiedHandlers[category]
	if proxified == nil {
		proxified = &ProxifiedHandler{
			routes: make(map[string]ProxyHandleFunc),
		}
		manager.proxifiedHandlers[category] = proxified
	}
	proxified.routes[command] = handleFunc

	return nil
}

// SetLogger sets the optional logger for this manager and all registered handlers.
func (manager *ProxyHandlers) SetLogger(logger *log.Logger) error {
	manager.logger = logger

	if manager.Interface != nil {
		if err := manager.Interface.SetLogger(logger); err != nil {
			return fmt.Errorf("proxy manager SetLogger: %w", err)
		}
	}

	for category, proxified := range manager.proxifiedHandlers {
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
func (manager *ProxyHandlers) Start() error {
	if manager.Interface == nil {
		return fmt.Errorf("proxy manager interface is nil, please create this manager using NewProxyHandlers(serviceName)")
	}
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
	if err := manager.Interface.Start(); err != nil {
		return fmt.Errorf("proxy manager Start: %w", err)
	}

	manager.running = true
	return nil
}

// Close closes all registered handlers.
func (manager *ProxyHandlers) Close() error {
	if manager.Interface == nil {
		return fmt.Errorf("proxy manager interface is nil, please create this manager using NewProxyHandlers(serviceName)")
	}

	for _, proxified := range manager.proxifiedHandlers {
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
			if err := closeHandlers([]base.Interface{proxified.handler}); err != nil {
				return err
			}
		}
	}

	if err := closeHandlers([]base.Interface{manager.Interface}); err != nil {
		return err
	}
	manager.running = false

	return nil
}

// Requires 'category' (string) parameter, returns 'exists' (boolean)
func (manager *ProxyHandlers) onIsProxyHandlerExist(req message.RequestInterface) message.ReplyInterface {
	category, err := req.RouteParameters().StringValue("category")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('category'): %v", err))
	}

	proxified := manager.proxifiedHandlers[Category(category)]
	exists := proxified != nil && proxified.handler != nil

	return req.Ok(datatype.New().Set("exists", exists))
}

// Requires 'category' (string) parameter, returns 'running' (boolean)
func (manager *ProxyHandlers) onIsProxyHandlerRunning(req message.RequestInterface) message.ReplyInterface {
	category, err := req.RouteParameters().StringValue("category")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('category'): %v", err))
	}

	proxified := manager.proxifiedHandlers[Category(category)]
	if proxified == nil || proxified.handler == nil {
		return req.Fail(fmt.Sprintf("No proxified handler was set, please call %s command to set it first", SetProxyHandlerCommand))
	}

	return req.Ok(datatype.New().Set("running", proxified.running))
}

// Requires 'category' (string) parameter, returns empty reply on success
func (manager *ProxyHandlers) onStartProxyHandler(req message.RequestInterface) message.ReplyInterface {
	category, err := req.RouteParameters().StringValue("category")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('category'): %v", err))
	}

	proxified := manager.proxifiedHandlers[Category(category)]
	if err := manager.startProxyHandler(proxified); err != nil {
		return req.Fail(err.Error())
	}

	return req.Ok(datatype.New())
}

func (manager *ProxyHandlers) onStartProxyHandlers(req message.RequestInterface) message.ReplyInterface {
	for category, proxified := range manager.proxifiedHandlers {
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
func (manager *ProxyHandlers) onStopProxyHandler(req message.RequestInterface) message.ReplyInterface {
	category, err := req.RouteParameters().StringValue("category")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('category'): %v", err))
	}

	proxified := manager.proxifiedHandlers[Category(category)]
	if err := manager.stopProxyHandler(proxified); err != nil {
		return req.Fail(err.Error())
	}

	return req.Ok(datatype.New())
}

func (manager *ProxyHandlers) onStopProxyHandlers(req message.RequestInterface) message.ReplyInterface {
	for category, proxified := range manager.proxifiedHandlers {
		if proxified == nil || !proxified.running {
			continue
		}
		if err := manager.stopProxyHandler(proxified); err != nil {
			return req.Fail(fmt.Sprintf("stop proxy handler(%s): %v", category, err))
		}
	}

	return req.Ok(datatype.New())
}

func (manager *ProxyHandlers) startProxyHandler(proxified *ProxifiedHandler) error {
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
	if err := proxified.handler.Start(); err != nil {
		return fmt.Errorf("proxified handler Start: %v", err)
	}
	proxified.running = true

	return nil
}

func (manager *ProxyHandlers) stopProxyHandler(proxified *ProxifiedHandler) error {
	if proxified == nil || proxified.handler == nil {
		return fmt.Errorf("No proxified handler was set, please call %s command to set it first", SetProxyHandlerCommand)
	}
	if !proxified.running {
		return fmt.Errorf("proxified handler is not running")
	}
	if err := closeHandlers([]base.Interface{proxified.handler}); err != nil {
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
func (manager *ProxyHandlers) onRemoveProxyHandler(req message.RequestInterface) message.ReplyInterface {
	category, err := req.RouteParameters().StringValue("category")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('category'): %v", err))
	}

	proxified := manager.proxifiedHandlers[Category(category)]
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
func (manager *ProxyHandlers) onSetProxyHandler(req message.RequestInterface) message.ReplyInterface {
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
	proxified := manager.proxifiedHandlers[category]
	if proxified == nil {
		proxified = &ProxifiedHandler{
			routes: make(map[string]ProxyHandleFunc),
		}
		manager.proxifiedHandlers[category] = proxified
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
	handler.SetConfig(handlerConfig.New(
		handlerConfig.HandlerType(proxyConfig.Type),
		proxyConfig.Endpoint.Id,
		proxyConfig.Category,
		proxyConfig.Endpoint.Port,
	))
	handler.SetPacker(manager)
	if manager.logger != nil {
		if err := handler.SetLogger(manager.logger); err != nil {
			return req.Fail(fmt.Sprintf("handler.SetLogger: %v", err))
		}
	}
	if err = handler.Route(base.Any, manager.handleFunc); err != nil {
		return req.Fail(fmt.Sprintf("Failed to route for proxifying (category: '%s').Route('%s'): %+v", category, base.Any, err))
	}

	proxified.handler = handler
	proxified.proxyConfig = proxyConfig
	proxified.outboundClients, err = manager.newOutboundClients(proxyConfig)
	if err != nil {
		return req.Fail(fmt.Sprintf("new outbound clients: %v", err))
	}
	proxified.running = false

	return req.Ok(datatype.New())
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

func dereferenceOutboundURL(url string) string {
	if url == "" {
		return url
	}
	var soil mushroom.Soil
	hypha, err := soil.Hypha(url)
	if err != nil || !hypha.URL {
		return url
	}
	return hypha.AsDereference().String()
}

func (manager *ProxyHandlers) resolveOutboundHandler(mushroomURL string) (topologyConfig.IndependentHandler, error) {
	topologyClient, err := topology.NewClient()
	if err != nil {
		return topologyConfig.IndependentHandler{}, err
	}
	defer topologyClient.Close()

	record, err := topologyClient.Handler(dereferenceOutboundURL(mushroomURL))
	if err != nil {
		return topologyConfig.IndependentHandler{}, err
	}
	handler, ok := record.AsIndependentHandler()
	if !ok {
		return topologyConfig.IndependentHandler{}, fmt.Errorf("outbound %q is not an independent handler", mushroomURL)
	}
	return handler, nil
}

func (manager *ProxyHandlers) newOutboundClients(proxyConfig topologyConfig.ProxyHandler) (map[string]outboundClient, error) {
	clients := make(map[string]outboundClient)
	for i, outboundURL := range proxyConfig.Outbounds {
		if outboundURL == "" {
			return nil, fmt.Errorf("outbounds[%d] url is required", i)
		}
		handler, err := manager.resolveOutboundHandler(outboundURL)
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
	var client outboundClient
	var err error

	switch handler.Type {
	case topologyConfig.SyncReplierType:
		client, err = clientSyncReplier.NewClient(handler.Endpoint.Id, handler.Endpoint.Port)
	case topologyConfig.ReplierType:
		client, err = clientReplier.NewClient(handler.Endpoint.Id, handler.Endpoint.Port)
	case topologyConfig.PublisherType:
		client, err = clientPublisher.NewClient(handler.Endpoint.Id, handler.Endpoint.Port)
		if err == nil {
			configureOutboundReceiver(client)
		}
	case topologyConfig.PairType:
		client, err = clientPair.NewClient(handler.Endpoint.Id, handler.Endpoint.Port)
	case topologyConfig.WorkerType:
		client, err = clientWorker.NewClient(handler.Endpoint.Id, handler.Endpoint.Port)
	default:
		return nil, fmt.Errorf("unsupported outbound handler type: %s", handler.Type)
	}
	if err != nil {
		return nil, err
	}
	return client, nil
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

func (manager *ProxyHandlers) defaultOutbound() (string, string, error) {
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

func (manager *ProxyHandlers) resolveOutboundRef(ref string) (string, string, error) {
	if ref == "" {
		return "", "", fmt.Errorf("outbound ref is empty")
	}

	for _, category := range manager.sortedProxifiedCategories() {
		proxified := manager.proxifiedHandlers[Category(category)]
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

func (manager *ProxyHandlers) sortedProxifiedCategories() []string {
	categories := make([]string, 0, len(manager.proxifiedHandlers))
	for category := range manager.proxifiedHandlers {
		categories = append(categories, string(category))
	}
	sort.Strings(categories)
	return categories
}

func (manager *ProxyHandlers) firstProxifiedHandler() (*ProxifiedHandler, error) {
	if len(manager.proxifiedHandlers) == 0 {
		return nil, fmt.Errorf("no proxified handlers")
	}

	for _, category := range manager.sortedProxifiedCategories() {
		proxified := manager.proxifiedHandlers[Category(category)]
		if proxified != nil && proxified.proxyConfig.Category != "" {
			return proxified, nil
		}
	}

	return nil, fmt.Errorf("no proxified handler configs")
}

func newProxyHandler(handlerType topologyConfig.HandlerType) (base.Interface, error) {
	switch handlerType {
	case topologyConfig.SyncReplierType:
		return sync_replier.New(), nil
	case topologyConfig.ReplierType:
		return replier.New(), nil
	case topologyConfig.PublisherType:
		return publisher.New(), nil
	case topologyConfig.PairType:
		return pair.New(), nil
	case topologyConfig.WorkerType:
		return worker.New(), nil
	default:
		return nil, fmt.Errorf("unsupported handler type: %s", handlerType)
	}
}

/****************************************************************************
 * ProxyHandlers also implements the message.Packer interface.
 * Although all messages within noPerfection must follow noPerfection/protocol/message.RequestInterface and noPerfection/protocol/message.ReplyInterface interfaces,
 * With the packers we can add a tail to them and within the structs like this ProxyHandler,
****************************************************************************/

func (manager *ProxyHandlers) DeserializeRequest(zmqEnvelope []string) (message.RequestInterface, error) {
	if err := message.ValidEnvelope(zmqEnvelope); err != nil {
		return nil, err
	}

	conId, msg, tail := message.EnvelopeToMessage(zmqEnvelope)

	data, err := datatype.NewFromString(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to convert message string %s to key-value: %v", msg, err)
	}

	var request message.Request
	err = data.Interface(&request)
	if err != nil {
		return nil, fmt.Errorf("failed to convert key-value %v to intermediate interface: %v", data, err)
	}

	if request.String() == "" {
		return nil, fmt.Errorf("failed to validate")
	}
	request.SetConId(conId)

	var proxifiedHandler string
	var outboundURL string
	if len(tail) > 0 {
		proxifiedHandler, outboundURL, err = manager.resolveOutboundRef(tail[0])
		if err != nil {
			return nil, err
		}
	} else {
		proxifiedHandler, outboundURL, err = manager.defaultOutbound()
		if err != nil {
			return nil, err
		}
	}

	return &ProxyRequest{
		Request:          request,
		proxifiedHandler: proxifiedHandler,
		outboundURL:      outboundURL,
		manager:          manager,
	}, nil
}

func (manager *ProxyHandlers) DeserializeReply(zmqEnvelope []string) (message.ReplyInterface, error) {
	if err := message.ValidEnvelope(zmqEnvelope); err != nil {
		return nil, err
	}

	conId, msg, _ := message.EnvelopeToMessage(zmqEnvelope)
	data, err := datatype.NewFromString(msg)
	if err != nil {
		return nil, fmt.Errorf("datatype.NewFromString: %w", err)
	}

	var reply message.Reply
	err = data.Interface(&reply)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize key-value to msg.Reply: %v", err)
	}
	reply.SetConId(conId)

	if reply.String() == "" {
		return nil, fmt.Errorf("validation failed")
	}

	return &ProxyReply{Reply: reply}, nil
}

func (manager *ProxyHandlers) SerializeRequest(request message.RequestInterface) ([]string, error) {
	str := request.String()
	if str == "" {
		return nil, fmt.Errorf("request.String returned an empty string")
	}

	if proxyRequest, ok := request.(*ProxyRequest); ok {
		if proxyRequest.outboundURL != "" {
			return message.MessageToEnvelope(request.ConId(), str, proxyRequest.outboundURL), nil
		}
	}

	return message.MessageToEnvelope(request.ConId(), str), nil
}

func (manager *ProxyHandlers) SerializeReply(reply message.ReplyInterface) ([]string, error) {
	str := reply.String()
	if str == "" {
		return nil, fmt.Errorf("request.String returned an empty string")
	}

	return message.MessageToEnvelope(reply.ConId(), str), nil
}

func (manager *ProxyHandlers) EmptyRequest() message.RequestInterface {
	return &ProxyRequest{}
}

func (manager *ProxyHandlers) EmptyReply() message.ReplyInterface {
	return &ProxyReply{}
}
