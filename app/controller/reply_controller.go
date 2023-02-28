/*
Controller package is the interface of the module.
It acts as the input receiver for other services or for external users.
*/
package controller

import (
	"errors"

	"github.com/charmbracelet/log"

	"github.com/blocklords/gosds/app/account"
	"github.com/blocklords/gosds/app/remote/message"
	"github.com/blocklords/gosds/app/service"
	"github.com/blocklords/gosds/db"

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
		return nil, err
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
	return err
}

// Controllers started to receive messages
func (c *Controller) Run(db_connection *db.Database, commands CommandHandlers) error {
	if err := c.socket.Bind("tcp://*:" + c.service.Port()); err != nil {
		return errors.New("error to bind socket for '" + c.service.ServiceName() + ": " + c.service.Port() + "' : " + err.Error())
	}

	if c.logger != nil {
		c.logger.Info("reply controller runs successfully", "port", c.service.Port())
	}

	for {
		// msg_raw, metadata, err := c.socket.RecvMessageWithMetadata(0, "pub_key")
		msg_raw, err := c.socket.RecvMessage(0)
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

		// Any request types is compatible with the Request.
		if !commands.Exist(request.Command) {
			fail := message.Fail("unsupported command " + request.Command)
			reply, _ := fail.ToString()
			if _, err := c.socket.SendMessage(reply); err != nil {
				return errors.New("invalid command message replying error: %w" + err.Error())
			}
			continue
		}

		var reply message.Reply

		// requester := account.New(metadata["pub_key"])

		// The command might be from a smartcontract developer.
		command_handler, ok := commands[request.Command]
		if !ok {
			// smartcontract_developer_request, err := message.ParseSmartcontractDeveloperRequest(msg_raw)
			// if err != nil {
			// 	fail := message.Fail("invalid smartcontract developer request " + err.Error())
			// 	reply, _ := fail.ToString()
			// 	if _, err := c.socket.SendMessage(reply); err != nil {
			// 		return errors.New("failed to reply: %w" + err.Error())
			// 	}
			// 	continue
			// }

			// smartcontract_developer, err := account.NewSmartcontractDeveloper(&smartcontract_developer_request)
			// if err != nil {
			// 	println(smartcontract_developer_request.NonceTimestamp)
			// 	fail := message.Fail("reply controller error as invalid smartcontract developer request: " + err.Error())
			// 	reply, _ := fail.ToString()
			// 	if _, err := c.socket.SendMessage(reply); err != nil {
			// 		return errors.New("failed to reply: %w" + err.Error())
			// 	}
			// 	continue
			// }

			// reply = command_handler(db_connection, smartcontract_developer_request, smartcontract_developer)
		} else {
			c.logger.Info("calling handler", "command", request.Command, "parameters", request.Parameters)
			reply = command_handler(db_connection, request, c.logger)
			c.logger.Info("command handled", "reply status", reply.Status)
		}

		reply_string, err := reply.ToString()
		c.logger.Info("reply back command result", "parameters", reply)
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
