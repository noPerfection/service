/*
Controller package is the interface of the module.
It acts as the input receiver for other services or for external users.
*/
package controller

import (
	"errors"

	"github.com/blocklords/gosds/app/account"
	"github.com/blocklords/gosds/app/remote/message"
	"github.com/blocklords/gosds/app/service"
	"github.com/blocklords/gosds/db"

	zmq "github.com/pebbe/zmq4"
)

type CommandHandlers map[string]interface{}

type Controller struct {
	service *service.Service
	socket  *zmq.Socket
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

// We set the whitelisted accounts that has access to this controller
func AddWhitelistedAccounts(s *service.Service, accounts account.Accounts) {
	zmq.AuthCurveAdd(s.ServiceName(), accounts.PublicKeys()...)
}

// Set the private key, so connected clients can identify this controller
func (c *Controller) SetControllerPrivateKey() error {
	err := c.socket.ServerAuthCurve(c.service.ServiceName(), c.service.SecretKey)
	return err
}

// Controllers started to receive messages
func (c *Controller) Run(db_connection *db.Database, commands CommandHandlers) error {
	if err := c.socket.Bind("tcp://*:" + c.service.Port()); err != nil {
		return errors.New("error to bind socket for '" + c.service.ServiceName() + ": " + c.service.Port() + "' : " + err.Error())
	}

	println("'" + c.service.ServiceName() + "' request-reply server runs on port " + c.service.Port())

	for {
		// msg_raw, metadata, err := socket.RecvMessageWithMetadata(0, "pub_key")
		msg_raw, err := c.socket.RecvMessage(0)
		if err != nil {
			fail := message.Fail("socket error to receive message " + err.Error())
			reply, _ := fail.ToString()
			if _, err := c.socket.SendMessage(reply); err != nil {
				return errors.New("failed to reply: %w" + err.Error())
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
				return errors.New("failed to reply: %w" + err.Error())
			}
			continue
		}

		// Any request types is compatible with the Request.
		if commands[request.Command] == nil {
			fail := message.Fail("unsupported command " + request.Command)
			reply, _ := fail.ToString()
			if _, err := c.socket.SendMessage(reply); err != nil {
				return errors.New("failed to reply: %w" + err.Error())
			}
			continue
		}

		var reply message.Reply

		// The command might be from a smartcontract developer.
		command_handler, ok := commands[request.Command].(func(*db.Database, message.SmartcontractDeveloperRequest, *account.SmartcontractDeveloper) message.Reply)
		if ok {
			smartcontract_developer_request, err := message.ParseSmartcontractDeveloperRequest(msg_raw)
			if err != nil {
				fail := message.Fail("invalid smartcontract developer request " + err.Error())
				reply, _ := fail.ToString()
				if _, err := c.socket.SendMessage(reply); err != nil {
					return errors.New("failed to reply: %w" + err.Error())
				}
				continue
			}

			smartcontract_developer, err := account.NewSmartcontractDeveloper(&smartcontract_developer_request)
			if err != nil {
				println(smartcontract_developer_request.NonceTimestamp)
				fail := message.Fail("reply controller error as invalid smartcontract developer request: " + err.Error())
				reply, _ := fail.ToString()
				if _, err := c.socket.SendMessage(reply); err != nil {
					return errors.New("failed to reply: %w" + err.Error())
				}
				continue
			}

			reply = command_handler(db_connection, smartcontract_developer_request, smartcontract_developer)
		} else {
			// The command might be from another SDS Service
			service_handler, ok := commands[request.Command].(func(*db.Database, message.ServiceRequest, *account.Account) message.Reply)
			if ok {
				service_request, err := message.ParseServiceRequest(msg_raw)
				if err != nil {
					fail := message.Fail("invalid service request " + err.Error())
					reply, _ := fail.ToString()
					if _, err := c.socket.SendMessage(reply); err != nil {
						return errors.New("failed to reply: %w" + err.Error())
					}
					continue
				}

				service_account := account.NewService(service_request.Service())

				reply = service_handler(db_connection, service_request, service_account)
			} else {
				// The command is from a developer.
				reply = commands[request.Command].(func(*db.Database, message.Request) message.Reply)(db_connection, request)
			}
		}

		reply_string, err := reply.ToString()
		if err != nil {
			if _, err := c.socket.SendMessage(err.Error()); err != nil {
				return errors.New("failed to reply: %w" + err.Error())
			}
		} else {
			if _, err := c.socket.SendMessage(reply_string); err != nil {
				return errors.New("failed to reply: %w" + err.Error())
			}
		}
	}
}
