package handlers

import (
	"fmt"
	"strings"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/log"
	"github.com/noPerfection/protocol/handler/base"
	handlerConfig "github.com/noPerfection/protocol/handler/config"
	"github.com/noPerfection/protocol/handler/pair"
	"github.com/noPerfection/protocol/handler/publisher"
	"github.com/noPerfection/protocol/handler/replier"
	"github.com/noPerfection/protocol/handler/sync_replier"
	"github.com/noPerfection/protocol/handler/worker"
	"github.com/noPerfection/protocol/message"
	topologyConfig "github.com/noPerfection/topology/config"
)

const (
	ProxyManagerCategory         = "_proxy_manager_noperf"
	SetProxyHandlerCommand       = "set-proxy-handler-command"
	IsProxyHandlerExistCommand   = "is-proxy-handler-exist-command"
	IsProxyHandlerRunningCommand = "is-proxy-handler-running-command"
	StartProxyHandlerCommand     = "start-proxy-handler-command"
	StopProxyHandlerCommand      = "stop-proxy-handler-command"
	RemoveProxyHandlerCommand    = "remove-proxy-handler-command"
)

// Proxy services work with a special type of requests and replies.
// And handles them in a special way: ProxyHandleFunc
// They are all following the message.RequestInterface and message.ReplyInterface interfaces.
// Where is ProxyHandleFunc is concrete case of noPerfection/protocol/handler/base.HandleFunc
type ProxyRequest struct {
	message.Request
}
type ProxyReply struct {
	message.Reply
}

type ProxyHandleFunc func(req ProxyRequest) ProxyReply

type Category string

type ProxifiedHandler struct {
	handler     base.Interface
	routes      map[string]ProxyHandleFunc  // user can do whatever he wants
	proxyConfig topologyConfig.ProxyHandler // handler's information
	running     bool
}

// ProxyHandlers owns the proxy handler registry and lifecycle.
// The proxy is a service, only difference from other types of services
// is using noPerfection/topology/config.ProxyHandler instead noPerfection/topology/config.Handler
type ProxyHandlers struct {
	base.Interface
	proxifiedHandlers map[Category]*ProxifiedHandler
	logger            *log.Logger
	running           bool
}

var _ message.ReplyInterface = (*ProxyReply)(nil)
var _ message.RequestInterface = (*ProxyRequest)(nil)
var _ message.Packer = (*ProxyHandlers)(nil)

// Proxy's Request functions
func (request ProxyRequest) Forward() (ProxyReply, error) {
	return ProxyReply{}, fmt.Errorf("todo not implemented")
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
		serviceName+ProxyManagerCategory,
		ProxyManagerCategory,
		0,
	))

	return &ProxyHandlers{
		Interface:         manager,
		proxifiedHandlers: make(map[Category]*ProxifiedHandler),
	}
}

/*
This is overwriting any handler's routes to go through the proxy.
*/
func (manager *ProxyHandlers) handleFunc(request message.RequestInterface) message.ReplyInterface {
	return request.Ok(request.RouteParameters().Set("proxified: todo need to go through routers", true))
}

func (manager *ProxyHandlers) Route(command string, handleFunc ProxyHandleFunc, handlerCategory ...string) error {
	if manager.running {
		return fmt.Errorf("I cant route when its already started. Please stop the handler first or the best way to route before starting the handler")
	}
	if len(handlerCategory) > 1 {
		return fmt.Errorf("too many handler categories")
	}

	category := Category(DefaultHandlerCategory)
	if len(handlerCategory) == 1 && handlerCategory[0] != "" {
		category = Category(handlerCategory[0])
	}
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

	handlers := make([]base.Interface, 0, len(manager.proxifiedHandlers)+1)
	handlers = append(handlers, manager.Interface)
	for _, proxified := range manager.proxifiedHandlers {
		if proxified.handler != nil {
			handlers = append(handlers, proxified.handler)
		}
	}

	if err := closeHandlers(handlers); err != nil {
		return err
	}
	for _, proxified := range manager.proxifiedHandlers {
		proxified.running = false
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
	if proxified == nil || proxified.handler == nil {
		return req.Fail(fmt.Sprintf("No proxified handler was set, please call %s command to set it first", SetProxyHandlerCommand))
	}
	if proxified.running {
		return req.Fail("proxified handler is already running")
	}
	if err := proxified.handler.Start(); err != nil {
		return req.Fail(fmt.Sprintf("proxified handler Start: %v", err))
	}
	proxified.running = true

	return req.Ok(datatype.New())
}

// Requires 'category' (string) parameter, returns empty reply on success
func (manager *ProxyHandlers) onStopProxyHandler(req message.RequestInterface) message.ReplyInterface {
	category, err := req.RouteParameters().StringValue("category")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('category'): %v", err))
	}

	proxified := manager.proxifiedHandlers[Category(category)]
	if proxified == nil || proxified.handler == nil {
		return req.Fail(fmt.Sprintf("No proxified handler was set, please call %s command to set it first", SetProxyHandlerCommand))
	}
	if !proxified.running {
		return req.Fail("proxified handler is not running")
	}
	if err := closeHandlers([]base.Interface{proxified.handler}); err != nil {
		return req.Fail(fmt.Sprintf("proxified handler Close: %v", err))
	}
	proxified.running = false

	return req.Ok(datatype.New())
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
	if len(proxyConfig.Routes) == 0 || len(proxified.routes) == 0 {
		return req.Fail("not possible to send since no routes are configured")
	}

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
	proxified.running = false

	return req.Ok(datatype.New())
}

func validateProxyHandlerOutbounds(proxyConfig topologyConfig.ProxyHandler) error {
	if len(proxyConfig.Outbounds) == 0 {
		return fmt.Errorf("not possible to send since no outbound yet")
	}

	for i, outbound := range proxyConfig.Outbounds {
		if outbound.Ref != "" {
			return fmt.Errorf("outbounds[%d] must be inline service, not ref %q", i, outbound.Ref)
		}
		if outbound.Service.IsZero() {
			return fmt.Errorf("outbounds[%d] service is required", i)
		}
		if len(outbound.Service.Handlers) == 0 {
			return fmt.Errorf("outbounds[%d] service %q must have at least one handler", i, outbound.Service.Name)
		}
		if err := topologyConfig.ValidateService(outbound.Service); err != nil {
			return fmt.Errorf("outbounds[%d] service: %w", i, err)
		}
	}

	return nil
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

	conId, msg, _ := message.EnvelopeToMessage(zmqEnvelope)

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

	return &ProxyRequest{Request: request}, nil
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
