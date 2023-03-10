/*
Controller package is the interface of the module.
It acts as the input receiver for other services or for external users.
*/
package controller

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/log"

	"github.com/blocklords/sds/app/account"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/app/service"
	"github.com/blocklords/sds/db"

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

// We set the whitelisted accounts that has access to this controller
func AddWhitelistedAccounts(s *service.Service, accounts account.Accounts) {
	zmq.AuthCurveAdd(s.Name, accounts.PublicKeys()...)
}

// Set the private key, so connected clients can identify this controller
// You call it before running the controller
func (c *Controller) SetControllerPrivateKey() error {
	err := c.socket.ServerAuthCurve(c.service.Name, c.service.SecretKey)
	if err == nil {
		return nil
	}
	return fmt.Errorf("ServerAuthCurve for domain %s: %w", c.service.Name, err)
}

// Controllers started to receive messages
func (c *Controller) Run(db_connection *db.Database, commands CommandHandlers) error {
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

		reply := commands[request.Command](db_connection, request, c.logger)

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
