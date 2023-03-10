/*
Controller package is the interface of the module.
It acts as the input receiver for other services or for external users.
*/
package controller

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/log"

	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/app/service"

	zmq "github.com/pebbe/zmq4"
)

type Controller struct {
	service *service.Service
	socket  *zmq.Socket
	logger  log.Logger
}

func NewReply(s *service.Service) (*Controller, error) {
	// Socket to talk to clients
	socket, err := zmq.NewSocket(zmq.REP)
	if err != nil {
		return nil, fmt.Errorf("zmq.NewSocket: %w", err)
	}

	return &Controller{
		socket:  socket,
		service: s,
	}, nil
}

// Set the logger
func (c *Controller) SetLogger(logger log.Logger) {
	c.logger = logger
}

// Controllers started to receive messages
// The parameters are the list of parameters that are passed to the command handlers
func (c *Controller) Run(commands CommandHandlers, parameters ...interface{}) error {
	if err := c.socket.Bind(c.service.Url()); err != nil {
		return fmt.Errorf("socket.bind on tcp protocol for %s at url %s: %w", c.service.Name, c.service.Url(), err)
	}

	if c.logger != nil {
		c.logger.Info("reply controller runs successfully", "url", c.service.Url())
	}

	for {
		msg_raw, metadata, err := c.socket.RecvMessageWithMetadata(0, "pub_key")
		if err != nil {
			fail := message.Fail("socket error to receive message " + err.Error())
			reply, _ := fail.ToString()
			if _, err := c.socket.SendMessage(reply); err != nil {
				return errors.New("recv error replying error %w" + err.Error())
			}
			continue
		}

		// All request types derive from the basic request.
		// We first attempt to parse basic request from the raw message
		request, err := message.ParseRequest(msg_raw)
		if err != nil {
			fail := message.Fail(err.Error())
			reply, _ := fail.ToString()
			if _, err := c.socket.SendMessage(reply); err != nil {
				return errors.New("parsing error replying error: %w" + err.Error())
			}
			continue
		}
		request.SetPublicKey(metadata["pub_key"])

		// Any request types is compatible with the Request.
		if !commands.Exist(request.Command) {
			fail := message.Fail("unsupported command " + request.Command)
			reply, _ := fail.ToString()
			if _, err := c.socket.SendMessage(reply); err != nil {
				return errors.New("invalid command message replying error: %w" + err.Error())
			}
			continue
		}

		reply := commands[request.Command](request, c.logger, parameters...)

		reply_string, err := reply.ToString()
		if err != nil {
			if _, err := c.socket.SendMessage(err.Error()); err != nil {
				return errors.New("converting reply to string %w" + err.Error())
			}
		} else {
			if _, err := c.socket.SendMessage(reply_string); err != nil {
				return errors.New("replying error %w" + err.Error())
			}
		}
	}
}
