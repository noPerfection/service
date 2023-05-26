package controller

import (
	"fmt"

	"github.com/blocklords/sds/app/communication/command"
	"github.com/blocklords/sds/app/communication/message"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/service"

	zmq "github.com/pebbe/zmq4"
)

// Controller is the socket wrapper for the service.
type Controller struct {
	service     *service.Service
	socket      *zmq.Socket
	logger      log.Logger
	socket_type zmq.Type
}

// NewReply creates a new synchrounous Reply controller.
func NewReply(s *service.Service, logger log.Logger) (*Controller, error) {
	if !s.IsThis() && !s.IsInproc() {
		return nil, fmt.Errorf("service should be limited to service.THIS or inproc type")
	}
	controller_logger, err := logger.Child("controller", "type", "reply", "service_name", s.Name, "inproc", s.IsInproc())

	if err != nil {
		return nil, fmt.Errorf("error creating child logger: %w", err)
	}

	// Socket to talk to clients
	socket, err := zmq.NewSocket(zmq.REP)
	if err != nil {
		return nil, fmt.Errorf("zmq.NewSocket: %w", err)
	}

	return &Controller{
		socket:      socket,
		service:     s,
		logger:      controller_logger,
		socket_type: zmq.REP,
	}, nil
}

func (c *Controller) is_repliable() bool {
	return c.socket_type == zmq.REP
}

// reply sends to the caller the message.
//
// If controller doesn't support replying (for example PULL controller)
// then it returns success.
func (c *Controller) reply(message message.Reply) error {
	if !c.is_repliable() {
		return nil
	}

	reply, _ := message.ToString()
	if _, err := c.socket.SendMessage(reply); err != nil {
		return fmt.Errorf("recv error replying error %w" + err.Error())
	}

	return nil
}

// Calls controller.reply() with the error message.
func (c *Controller) reply_error(err error) error {
	return c.reply(message.Fail(err.Error()))
}

// Run the controller.
//
// It will bind itself to the socket endpoint and waits for the message.Request.
// If message.Request.Command is defined in the handlers, then executes it.
//
// Valid call:
//
//		reply, _ := controller.NewReply(service, reply)
//	 	go reply.Run(handlers, database) // or reply.Run(handlers)
//
// The parameters are the list of parameters that are passed to the command handlers
func (c *Controller) Run(handlers command.Handlers, parameters ...interface{}) error {
	// if secure and not inproc
	// then we add the domain name of controller to the security layer
	//
	// then any whitelisting users will be send there.
	if err := c.socket.Bind(c.service.Url()); err != nil {
		return fmt.Errorf("socket.bind on tcp protocol for %s at url %s: %w", c.service.Name, c.service.Url(), err)
	}

	for {
		msg_raw, metadata, err := c.socket.RecvMessageWithMetadata(0, "pub_key")
		if err != nil {
			new_err := fmt.Errorf("socket.recvMessageWithMetadata: %w", err)
			if err := c.reply_error(new_err); err != nil {
				return err
			}
			return new_err
		}

		// All request types derive from the basic request.
		// We first attempt to parse basic request from the raw message
		request, err := message.ParseRequest(msg_raw)
		if err != nil {
			new_err := fmt.Errorf("message.ParseRequest: %w", err)
			if err := c.reply_error(new_err); err != nil {
				return err
			}
			continue
		}
		request.SetPublicKey(metadata["pub_key"])

		request_command := command.New(request.Command)

		// Any request types is compatible with the Request.
		if !handlers.Exist(request_command) {
			new_err := fmt.Errorf("handler not found for command: %s", request.Command)
			if err := c.reply_error(new_err); err != nil {
				return err
			}
			continue
		}

		// for puller's it returns an error that occured on the blockchain.
		reply := handlers[request_command](request, c.logger, parameters...)
		if err := c.reply(reply); err != nil {
			return err
		}
		if !reply.IsOK() && !c.is_repliable() {
			c.logger.Warn("handler replied an error", "command", request.Command, "request parameters", request.Parameters, "error message", reply.Message)
		}
	}
}
