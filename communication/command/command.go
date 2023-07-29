package command

import (
	"fmt"
	"github.com/ahmetson/service-lib/log"
	"sync"

	"github.com/ahmetson/common-lib/data_type"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/service-lib/communication/message"
	parameter "github.com/ahmetson/service-lib/identity"
	"github.com/ahmetson/service-lib/remote"

	zmq "github.com/pebbe/zmq4"
)

// Route is the command, handler of the command
// and the extensions that this command depends on.
type Route struct {
	Command    string
	Extensions []string
	handler    HandleFunc
}

// Any command name
const Any string = "*"

// NewRoute returns a new command handler. It's used by the controllers.
func NewRoute(command string, handler HandleFunc, extensions ...string) *Route {
	return &Route{
		Command:    command,
		Extensions: extensions,
		handler:    handler,
	}
}

// AddHandler if the handler already exists then it will throw an error
func (route *Route) AddHandler(handler HandleFunc) error {
	if route.handler == nil {
		route.handler = handler
		return nil
	}

	return fmt.Errorf("handler exists in %s route", route.Command)
}

// FilterExtensionClients returns the list of the clients specific for this command
func (route *Route) filterExtensionClients(clients remote.Clients) []*remote.ClientSocket {
	routeClients := make([]*remote.ClientSocket, len(route.Extensions))

	added := 0
	for extensionName := range clients {
		for i := 0; i < len(route.Extensions); i++ {
			if route.Extensions[i] == extensionName {
				routeClients[added] = clients[extensionName].(*remote.ClientSocket)
				added++
			}
		}
	}

	return routeClients
}

func (route *Route) Handle(request message.Request, logger *log.Logger, allExtensions remote.Clients) message.Reply {
	extensions := route.filterExtensionClients(allExtensions)
	return route.handler(request, logger, extensions...)
}

// Request the command to the remote thread or service with the
// given request parameters via the socket.
//
// The response of the remote service is assigned to the reply.
//
// The reply should be passed by pointer.
//
// Example:
//
//		request_parameters := key_value.Empty()
//		var reply_parameters key_value.Empty()
//		ping_command := New("PING") // create a command
//	    // Send PING command to the socket.
//		_ := ping_command.Request(socket, request_parameters, &reply_parameters)
//		pong, _ := reply_parameters.GetString("pong")
func (route *Route) Request(socket *remote.ClientSocket, request interface{}, reply interface{}) error {
	_, ok := request.(message.Request)
	if ok {
		return fmt.Errorf("the request can not be of message.Request type")
	}
	_, ok = request.(message.SmartcontractDeveloperRequest)
	if ok {
		return fmt.Errorf("the request can not be of message.SmartcontractDeveloperRequest type")
	}

	_, ok = reply.(message.Reply)
	if ok {
		return fmt.Errorf("the reply can not be of message.Reply type")
	}
	_, ok = reply.(message.Broadcast)
	if ok {
		return fmt.Errorf("the reply can not be of message.Broadcast type")
	}
	if !data_type.IsPointer(reply) {
		return fmt.Errorf("the reply is not passed by pointer")
	}

	requestParameters, err := key_value.NewFromInterface(request)
	if err != nil {
		return fmt.Errorf("convert parameters to: %w", err)
	}

	requestMessage := message.Request{
		Command:    route.Command,
		Parameters: requestParameters,
	}

	replyParameters, err := socket.RequestRemoteService(&requestMessage)
	if err != nil {
		return fmt.Errorf("socket.RequestRemoteService: %w", err)
	}

	err = replyParameters.Interface(reply)
	if err != nil {
		return fmt.Errorf("reply.Parameters.ToInterface: %w", err)
	}

	return nil
}

// Push the command to the remote thread or service with the
// given request parameters via the socket.
//
// The Push is equivalent of Request without waiting for the remote socket's response.
//
// Example:
//
//			request_parameters := key_value.Empty().
//	         Set("timestamp", 1)
//			heartbeat := New("HEARTBEAT") // create a command
//		    // Send HEARTBEAT command to the socket.
//			_ := heartbeat.Request(socket, request_parameters)
//			server_timestamp, _ := reply_parameters.GetUint64("server_timestamp")
func (route *Route) Push(socket *zmq.Socket, request interface{}) error {
	socketType, err := socket.GetType()
	if err != nil {
		return fmt.Errorf("socket.GetType: %w", err)
	}
	if socketType != zmq.PUSH {
		return fmt.Errorf("socket type %s not supported. Only is supported PUSH", socketType)
	}

	_, ok := request.(message.Request)
	if ok {
		return fmt.Errorf("the request can not be of message.Request type")
	}
	_, ok = request.(message.SmartcontractDeveloperRequest)
	if ok {
		return fmt.Errorf("the request can not be of message.SmartcontractDeveloperRequest type")
	}

	var mu sync.Mutex
	requestParameters, err := key_value.NewFromInterface(request)
	if err != nil {
		return fmt.Errorf("convert parameters to: %w", err)
	}

	requestMessage := message.Request{
		Command:    route.Command,
		Parameters: requestParameters,
	}

	requestString, err := requestMessage.String()
	if err != nil {
		return fmt.Errorf("failed to stringify message: %w", err)
	}

	mu.Lock()
	_, err = socket.SendMessage(requestString)
	mu.Unlock()
	if err != nil {
		return fmt.Errorf("failed to send to blockchain package: %w", err)
	}

	return nil
}

// RequestRouter sends the command to the remote thread or service that over the proxy.
// The socket parameter is the proxy/broker socket.
// The service type is the service name that will accept the requests and response the reply.
//
// The reply parameter must be passed by pointer.
//
// In SeascapeSDS terminology, we call the proxy/broker as Router.
//
// Example:
//
//	        var reply key_value.KeyValue
//			request_parameters := key_value.Empty().
//		        Set("gold", 123)
//			set := New("SET") // create a command
//	        db_service := parameter.DB
//			// Send SET command to the database via the authentication proxy.
//			_ := set.RequestRouter(auth_socket, db_service, request_parameters, &reply_parameters)
func (route *Route) RequestRouter(socket *remote.ClientSocket, targetService *parameter.Service, request interface{}, reply interface{}) error {
	_, ok := request.(message.Request)
	if ok {
		return fmt.Errorf("the request can not be of message.Request type")
	}
	_, ok = request.(message.SmartcontractDeveloperRequest)
	if ok {
		return fmt.Errorf("the request can not be of message.SmartcontractDeveloperRequest type")
	}

	_, ok = reply.(message.Reply)
	if ok {
		return fmt.Errorf("the reply can not be of message.Reply type")
	}
	_, ok = reply.(message.Broadcast)
	if ok {
		return fmt.Errorf("the reply can not be of message.Broadcast type")
	}
	if !data_type.IsPointer(reply) {
		return fmt.Errorf("the reply must be passed by pointer")
	}

	requestParameters, err := key_value.NewFromInterface(request)
	if err != nil {
		return fmt.Errorf("convert parameters to: %w", err)
	}

	requestMessage := message.Request{
		Command:    route.Command,
		Parameters: requestParameters,
	}

	replyParameters, err := socket.RequestRouter(targetService, &requestMessage)
	if err != nil {
		return fmt.Errorf("socket.RequestRemoteService: %w", err)
	}

	err = replyParameters.Interface(reply)
	return err
}

// Reply creates a successful message.Reply with the given reply parameters.
func Reply(reply interface{}) (message.Reply, error) {
	replyParameters, err := key_value.NewFromInterface(reply)
	if err != nil {
		return message.Reply{}, fmt.Errorf("failed to encode reply: %w", err)
	}

	return message.Reply{
		Status:     message.OK,
		Message:    "",
		Parameters: replyParameters,
	}, nil
}
