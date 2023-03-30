package command

import (
	"fmt"
	"sync"

	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/app/service"
	"github.com/blocklords/sds/common/data_type/key_value"

	zmq "github.com/pebbe/zmq4"
)

type Command string

func (c Command) String() string {
	return string(c)
}

func New(value string) Command {
	return Command(value)
}

// Makes a remote request with the @request parameters
// And then returns the @reply.
//
// Both request and reply are the message parameters.
func (command Command) Request(socket *remote.Socket, request interface{}, reply interface{}) error {
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

	request_parameters, err := key_value.NewFromInterface(request)
	if err != nil {
		return fmt.Errorf("conver parameters to: %w", err)
	}

	request_message := message.Request{
		Command:    command.String(),
		Parameters: request_parameters,
	}

	reply_parameters, err := socket.RequestRemoteService(&request_message)
	if err != nil {
		return fmt.Errorf("socket.RequestRemoteService: %w", err)
	}

	err = reply_parameters.ToInterface(reply)
	return err
}

// Makes a remote request with the @request parameters
// And then returns the @reply.
//
// Both request and reply are the message parameters.
func (command Command) Push(socket *zmq.Socket, request interface{}) error {
	socket_type, err := socket.GetType()
	if err != nil {
		return fmt.Errorf("socket.GetType: %w", err)
	}
	if socket_type != zmq.PUSH {
		return fmt.Errorf("socket type %s not supported. Only is supported PUSH", socket_type)
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
	request_parameters, err := key_value.NewFromInterface(request)
	if err != nil {
		return fmt.Errorf("conver parameters to: %w", err)
	}

	request_message := message.Request{
		Command:    command.String(),
		Parameters: request_parameters,
	}

	request_string, err := request_message.ToString()
	if err != nil {
		return fmt.Errorf("failed to stringify message: %w", err)
	}

	mu.Lock()
	_, err = socket.SendMessage(request_string)
	mu.Unlock()
	if err != nil {
		return fmt.Errorf("failed to send to blockchain package: %w", err)
	}

	return nil
}

func (command Command) RequestRouter(socket *remote.Socket, service_type service.ServiceType, request interface{}, reply interface{}) error {
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

	request_parameters, err := key_value.NewFromInterface(request)
	if err != nil {
		return fmt.Errorf("conver parameters to: %w", err)
	}

	request_message := message.Request{
		Command:    command.String(),
		Parameters: request_parameters,
	}

	reply_parameters, err := socket.RequestRouter(service_type, &request_message)
	if err != nil {
		return fmt.Errorf("socket.RequestRemoteService: %w", err)
	}

	err = reply_parameters.ToInterface(reply)
	return err
}

func Reply(reply interface{}) (message.Reply, error) {
	reply_parameters, err := key_value.NewFromInterface(reply)
	if err != nil {
		return message.Reply{}, fmt.Errorf("failed to encode reply: %w", err)
	}

	return message.Reply{
		Status:     message.OK,
		Message:    "",
		Parameters: reply_parameters,
	}, nil
}
