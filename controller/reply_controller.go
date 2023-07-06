package controller

import (
	"fmt"

	"github.com/Seascape-Foundation/sds-service-lib/communication/command"
	"github.com/Seascape-Foundation/sds-service-lib/communication/message"
	"github.com/Seascape-Foundation/sds-service-lib/identity"
	"github.com/Seascape-Foundation/sds-service-lib/log"

	zmq "github.com/pebbe/zmq4"
)

// Controller is the socket wrapper for the service.
type Controller struct {
	service    *identity.Service
	socket     *zmq.Socket
	logger     log.Logger
	socketType zmq.Type
}

// NewReply creates a new synchronous Reply controller.
func NewReply(s *identity.Service, logger log.Logger) (*Controller, error) {
	if !s.IsThis() && !s.IsInproc() {
		return nil, fmt.Errorf("service should be limited to parameter.THIS or inproc type")
	}
	controllerLogger, err := logger.Child("controller", "type", "reply", "service_name", s.Name, "inproc", s.IsInproc())

	if err != nil {
		return nil, fmt.Errorf("error creating child logger: %w", err)
	}

	// Socket to talk to clients
	socket, err := zmq.NewSocket(zmq.REP)
	if err != nil {
		return nil, fmt.Errorf("zmq.NewSocket: %w", err)
	}

	return &Controller{
		socket:     socket,
		service:    s,
		logger:     controllerLogger,
		socketType: zmq.REP,
	}, nil
}

func (c *Controller) isReply() bool {
	return c.socketType == zmq.REP
}

// reply sends to the caller the message.
//
// If controller doesn't support replying (for example PULL controller)
// then it returns success.
func (c *Controller) reply(message message.Reply) error {
	if !c.isReply() {
		return nil
	}

	reply, _ := message.String()
	if _, err := c.socket.SendMessage(reply); err != nil {
		return fmt.Errorf("recv error replying error %w" + err.Error())
	}

	return nil
}

// Calls controller.reply() with the error message.
func (c *Controller) replyError(err error) error {
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
	// then any whitelisting users will be sent there.
	if err := c.socket.Bind(c.service.Url()); err != nil {
		return fmt.Errorf("socket.bind on tcp protocol for %s at url %s: %w", c.service.Name, c.service.Url(), err)
	}

	for {
		msgRaw, metadata, err := c.socket.RecvMessageWithMetadata(0, "pub_key")
		if err != nil {
			newErr := fmt.Errorf("socket.recvMessageWithMetadata: %w", err)
			if err := c.replyError(newErr); err != nil {
				return err
			}
			return newErr
		}

		// All request types derive from the basic request.
		// We first attempt to parse basic request from the raw message
		request, err := message.ParseRequest(msgRaw)
		if err != nil {
			newErr := fmt.Errorf("message.ParseRequest: %w", err)
			if err := c.replyError(newErr); err != nil {
				return err
			}
			continue
		}
		request.SetPublicKey(metadata["pub_key"])

		requestCommand := command.New(request.Command)

		// Any request types is compatible with the Request.
		if !handlers.Exist(requestCommand) {
			newErr := fmt.Errorf("handler not found for command: %s", request.Command)
			if err := c.replyError(newErr); err != nil {
				return err
			}
			continue
		}

		// for puller's it returns an error that occurred on the blockchain.
		reply := handlers[requestCommand](request, c.logger, parameters...)
		if err := c.reply(reply); err != nil {
			return err
		}
		if !reply.IsOK() && !c.isReply() {
			c.logger.Warn("handler replied an error", "command", request.Command, "request parameters", request.Parameters, "error message", reply.Message)
		}
	}
}
