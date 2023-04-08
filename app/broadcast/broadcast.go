/*
Package broadcast creates a sub process that can publish data to the external world.

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

	// remove the dependency on security/credentials
	"github.com/blocklords/sds/security/credentials"

	"github.com/blocklords/sds/app/remote/message"

	"github.com/blocklords/sds/app/log"

	zmq "github.com/pebbe/zmq4"
)

// NEW_MESSAGE is the command that broadcast accepts within the app.
// Then the message that it accept it will broadcast to the external world.
const NEW_MESSAGE command.CommandName = "new-message"

// Broadcast keeps the Service and PUB socket.
type Broadcast struct {
	service *service.Service
	socket  *zmq.Socket
	logger  log.Logger
}

// Prefix for logging
func broadcast_domain(s *service.Service) string {
	return s.Name + "_broadcast"
}

// New Broadcast for the given s service.Service.
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

// Security: we set the whitelisted account public keys that can subscribe to Broadcast
func AddWhitelistedAccounts(s *service.Service, public_keys []string) {
	zmq.AuthCurveAdd(broadcast_domain(s), public_keys...)
}

// Security: Set the CURVE private key for this broadcast.
// Run this function before you call broadcast.Run()
func (c *Broadcast) SetPrivateKey(service_credentials *credentials.Credentials) error {
	err := service_credentials.SetSocketAuthCurve(c.socket, broadcast_domain(c.service))
	if err != nil {
		return fmt.Errorf("socket.ServerAuthCurve: %w", err)
	}
	return nil
}

// ConnectionUrl returns url endpoint of broadcaast thread.
// Send the request with NEW_MESSAGE command to broadcast new message to the external world.
func ConnectionUrl(service *service.Service) string {
	return "inproc://broadcast_" + service.Name
}

// ConnectionSocket returns a socket that accessed to the broadcast thread.
// Send the NEW_MESSAGE command to this socket to broadcaster a new message to the external world.
func ConnectionSocket(service *service.Service) (*zmq.Socket, error) {
	sock, err := zmq.NewSocket(zmq.PUSH)
	if err != nil {
		return nil, fmt.Errorf("zmq error for new push socket: %w", err)
	}

	url := ConnectionUrl(service)
	if err != nil {
		return nil, fmt.Errorf("ConnectionUrl: %w", err)
	}
	if err := sock.Connect(url); err != nil {
		return nil, fmt.Errorf("trying to create a connection socket: %w", err)
	}

	return sock, nil
}

// Run Broadcast will create two sockets.
//   - PUB to publish the messages to the external world.
//     The parameters of the PUB is derived from Service when Broadcast was created.
//   - PULL is the controller binded to the ConnectionUrl endpoint.
//     This controller is used to accept the messages from other threads.
//     Once the messages are received, the Broadcast will redirect them to PUB socket.
//
// In case of error, it will exit entire application.
//
// Run it as a goroutine.
//
// Example:
//
//	// valid way to call
//	go b.Run()
//	// invalid way to call
//	b.Run()
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
	go func() {
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
	}()
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
