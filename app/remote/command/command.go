package command

import (
	"fmt"

	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/app/service"
	"github.com/blocklords/sds/common/data_type/key_value"
)

type Command string

func (c Command) String() string {
	return string(c)
}

// Makes a remote request with the @request parameters
// And then returns the @reply.
//
// Both request and reply are the message parameters.
func (command Command) Request(socket *remote.Socket, request interface{}, reply interface{}) error {
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

	err = reply_parameters.ToInterface(&reply)
	return err
}

func (command Command) RequestRouter(socket *remote.Socket, service_type service.ServiceType, request interface{}, reply interface{}) error {
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

	err = reply_parameters.ToInterface(&reply)
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
