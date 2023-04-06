/*
The Broadcast messages from a service,
subscribe to consume them.

The Broadcast depends on the service.Service
with the service.BROADCAST limit.

The service.BROADCAST limit requires SERVICE_BROADCAST_PORT
and SERVICE_BROADCAST_HOST configuration parameters.

Example to start broadcaster

	logger, _ := log.New("service", log.WITH_TIMESTAMP)
	service, _ := service.NewExternal(service.CATEGORIZER, service.BROADCAST)
	broadcast, _ := broadcast.New(service, logger)

	// Start the broadcaster
	go broadcast.Run()

	broadcast_connection, _ := broadcast.ConnectionSocket()
	msg := message.Broadcast{}
	msg_string, _ := msg.ToString()
	broadcast_connection.SendMessage(msg_string)

Example to subscribe

	app_config, _ := configuration.NewAppConfig()
	logger, _ := logger.New("client", log.WITHOUT_TIMESTAMP)
	service, _ := service.NewExternal(service.CATEGORIZER, service.SUBCRIBE)
	socket, _ := remote.NewTcpSubscriber(service, "topic", logger, app_config)
	while {
		// message.Broadcast, error
		broadcast_message, err := service.Subscribe()
		if err != nil {
			panic(err)
		}
	}
*/
package broadcast

import (
	"fmt"
	"sync"

	"github.com/blocklords/sds/app/command"
	"github.com/blocklords/sds/app/controller"
	"github.com/blocklords/sds/app/service"

	"github.com/blocklords/sds/app/remote/message"

	"github.com/blocklords/sds/app/log"

	zmq "github.com/pebbe/zmq4"
)

const NEW_MESSAGE command.Command = "new-message"

// Broadcast
type Broadcast struct {
	service *service.Service
	socket  *zmq.Socket
	logger  log.Logger
}

// Prefix for logging
func broadcast_domain(s *service.Service) string {
	return s.Name + "_broadcast"
}

// Starts a new broadcaster in the background
// The first parameter is the way to publish the messages.
// The second parameter starts the message
func New(s *service.Service, logger log.Logger) (*Broadcast, error) {
	if !s.IsBroadcast() {
		return nil, fmt.Errorf("the service is not limited to BROADCAST. run service.NewExternal(type, service.BROADCAST)")
	}

	logger, err := logger.ChildWithTimestamp("broadcast")
	if err != nil {
		return nil, fmt.Errorf("error creating child logger: %w", err)
	}

	broadcast := Broadcast{
		service: s,
		logger:  logger,
	}

	return &broadcast, nil
}

// We set the whitelisted accounts that has access to this controller
func AddWhitelistedAccounts(s *service.Service, public_keys []string) {
	zmq.AuthCurveAdd(broadcast_domain(s), public_keys...)
}

// Set the private key, so connected clients can identify this controller
// You call it before running the controller
func (c *Broadcast) SetPrivateKey() error {
	err := c.service.Credentials.SetSocketAuthCurve(c.socket, broadcast_domain(c.service))
	if err != nil {
		return fmt.Errorf("socket.ServerAuthCurve: %w", err)
	}
	return nil
}

// Returns the connection url if
// Broadcast is running.
func ConnectionUrl(service *service.Service) string {
	return "inproc://broadcast_" + service.Name
}

// Creates a socket that will send to the
// Broadcaster a new message.
func ConnectionSocket(service *service.Service) (*zmq.Socket, error) {
	sock, err := zmq.NewSocket(zmq.PUSH)
	if err != nil {
		return nil, fmt.Errorf("zmq error for new push socket: %w", err)
	}

	url := ConnectionUrl(service)
	if err != nil {
		return nil, fmt.Errorf("ConnectionUrl: %w", err)
	}
	if err := sock.Bind(url); err != nil {
		return nil, fmt.Errorf("trying to create a connection socket: %w", err)
	}

	return sock, nil
}

// Run a new broadcaster
//
// It assumes that the another package is starting an authentication layer of zmq:
// ZAP.
//
// # If some error is encountered, then this package panics
//
// use controller.Controller through controller.NewPull
func (b *Broadcast) Run() {
	// Socket to talk to clients
	broadcast_socket, err := zmq.NewSocket(zmq.PUB)
	if err != nil {
		b.logger.Fatal("zmq.NewSocket: %w", err)
	}
	b.socket = broadcast_socket

	err = b.socket.Bind(b.service.Url())
	if err != nil {
		b.logger.Fatal("could not listen to publisher", "broadcast_url", b.service.Url(), "message", err)
	}

	url := ConnectionUrl(b.service)
	pull_service, err := service.InprocessFromUrl(url)
	if err != nil {
		b.logger.Fatal("could not create inprocess service for broadcast", "broadcast url", b.service.Url(), "puller url", url, "error", err)
	}
	pull, err := controller.NewPull(pull_service, b.logger)
	if err != nil {
		b.logger.Fatal("could not pull controller for broadcast", "broadcast url", b.service.Url(), "puller url", url, "error", err)
	}

	handlers := command.EmptyHandlers().
		Add(NEW_MESSAGE, on_new_message)

	err = pull.Run(handlers, b)
	if err != nil {
		b.logger.Fatal("pull.Run", "broadcast url", b.service.Url(), "puller url", url, "error", err)
	}
}

func on_new_message(request message.Request, _ log.Logger, parameters ...interface{}) message.Reply {
	if parameters == nil || len(parameters) < 1 {
		return message.Fail("invalid parameters were given atleast broadcast should be passed")
	}

	broadcast, ok := parameters[0].(*Broadcast)
	if !ok {
		return message.Fail("missing Manager in the parameters")
	}

	topic, err := request.Parameters.GetString("topic")
	if err != nil {
		return message.Fail("parameters.GetString(`topic`):" + err.Error())
	}

	reply_parameters, err := request.Parameters.GetKeyValue("reply_parameters")
	if err != nil {
		return message.Fail("parameters.GetString(`topic`):" + err.Error())
	}

	reply := message.Reply{
		Status:     message.OK,
		Message:    "",
		Parameters: reply_parameters,
	}
	broadcast_message := message.NewBroadcast(topic, reply)

	var mu sync.Mutex
	broadcast_bytes := broadcast_message.ToBytes()
	mu.Lock()
	_, err = broadcast.socket.SendMessage(topic, broadcast_bytes)
	mu.Unlock()
	if err != nil {
		broadcast.logger.Fatal("socket error to send message", "message", err)
	}

	return reply
}
