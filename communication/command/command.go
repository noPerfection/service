package command

import (
	"fmt"
	"sync"

	"github.com/Seascape-Foundation/sds-common-lib/data_type"
	"github.com/Seascape-Foundation/sds-common-lib/data_type/key_value"
	"github.com/Seascape-Foundation/sds-service-lib/communication/message"
	parameter "github.com/Seascape-Foundation/sds-service-lib/identity"
	"github.com/Seascape-Foundation/sds-service-lib/remote"

	zmq "github.com/pebbe/zmq4"
)

// Name is the string
// It's included in the message.Request when another thread or user requests
// the SDS Service
type Name string

// String representation of the CommandName
func (command Name) String() string {
	return string(command)
}

// New Converts the given string to the CommandName
func New(value string) Name {
	return Name(value)
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
func (command Name) Request(socket *remote.ClientSocket, request interface{}, reply interface{}) error {
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
		Command:    command.String(),
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
func (command Name) Push(socket *zmq.Socket, request interface{}) error {
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
		Command:    command.String(),
		Parameters: requestParameters,
	}

	requestString, err := requestMessage.ToString()
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
func (command Name) RequestRouter(socket *remote.ClientSocket, targetService *parameter.Service, request interface{}, reply interface{}) error {
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
		Command:    command.String(),
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
