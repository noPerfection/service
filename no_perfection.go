package service

import (
	"fmt"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/protocol/client"
	"github.com/noPerfection/protocol/handler/base"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service/handlers"
	"github.com/noPerfection/topology"
	topologyConfig "github.com/noPerfection/topology/config"
)

type (
	// RequestInterface is the request passed to route handlers.
	RequestInterface = message.RequestInterface
	// ReplyInterface is the reply returned from route handlers.
	ReplyInterface = message.ReplyInterface
	// HandlerType is the client handler protocol to connect to.
	HandlerType        = topologyConfig.HandlerType
	Handler            = topologyConfig.Handler
	IndependentHandler = topologyConfig.IndependentHandler
	ProxyHandler       = topologyConfig.ProxyHandler
	ExtensionHandler   = topologyConfig.ExtensionHandler
	Config             = topologyConfig.Service
	Dependency         = topologyConfig.DepService
	ProxyRequest       = handlers.ProxyRequest
	ProxyReply         = handlers.ProxyReply
)

// Service can be Proxy, Independent, or Extension.
type Service interface {
	isService()
	AsIndependent() (*Independent, bool)
	AsProxy() (*Proxy, bool)
	AsExtension() (*Extension, bool)
	Start() error
	Stop() error
}

var (
	_ Service = (*Independent)(nil)
	_ Service = (*Proxy)(nil)
	_ Service = (*Extension)(nil)
)

const (
	// Service types
	IndependentType = topologyConfig.IndependentType
	ProxyType       = topologyConfig.ProxyType
	ExtensionType   = topologyConfig.ExtensionType
	// Handler types
	SyncReplierType = topologyConfig.SyncReplierType
	PublisherType   = topologyConfig.PublisherType
	ReplierType     = topologyConfig.ReplierType
	PairType        = topologyConfig.PairType
	WorkerType      = topologyConfig.WorkerType
	//--
	AnyCmd                 = base.Any
	DefaultHandlerCategory = handlers.DefaultHandlerCategory
	ServiceManagerCategory = topology.ServiceManagerCategory
)

// Create a new import. Alias for noPerfection/protocol/message.NewEndpoint
var Endpoint = message.NewEndpoint
var KeyValue = datatype.New

// Client connects to a service handler. All parameters are optional.
// Pass id (string), port (uint, uint64, or int), and handler type (HandlerType) in any order.
// Defaults: localhost, 8000, ReplierType.
func Client(params ...any) (*client.Socket, error) {
	id := handlers.DefaultHandlerEndpoint.Id
	port := handlers.DefaultHandlerEndpoint.Port
	handlerType := ReplierType

	for _, param := range params {
		switch value := param.(type) {
		case string:
			id = value
		case HandlerType:
			handlerType = value
		case uint64:
			port = value
		case uint:
			port = uint64(value)
		case int:
			if value < 0 {
				return nil, fmt.Errorf("port must be non-negative")
			}
			port = uint64(value)
		default:
			return nil, fmt.Errorf("unsupported client parameter type %T", param)
		}
	}

	return client.New(id, port, client.HandlerType(handlerType))
}

// RequestMsg builds a client request. parameters are optional.
// Pass map[string]any{...} or datatype.KeyValue. Returns nil when parameters are invalid.
func RequestMsg(cmd string, parameters ...any) RequestInterface {
	var err error
	var kv datatype.KeyValue
	if len(parameters) == 0 || parameters[0] == nil {
		kv = datatype.New()
	} else {
		switch params := parameters[0].(type) {
		case datatype.KeyValue:
			kv = params
		default:
			kv, err = datatype.NewFromInterface(params)
			if err != nil {
				return nil
			}
		}
	}
	return &message.Request{
		Command:    cmd,
		Parameters: kv,
	}
}
