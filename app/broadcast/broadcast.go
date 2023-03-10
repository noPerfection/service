// Broadcast package creates a publishing socket
// Use this package in a goroutine.
package broadcast

import (
	"fmt"
	"sync"

	"github.com/blocklords/sds/app/account"
	"github.com/blocklords/sds/app/service"

	"github.com/blocklords/sds/app/remote/message"

	app_log "github.com/blocklords/sds/app/log"
	"github.com/charmbracelet/log"

	zmq "github.com/pebbe/zmq4"
)

// Broadcast
type Broadcast struct {
	service *service.Service
	socket  *zmq.Socket
	logger  log.Logger
	In      chan message.Broadcast
}

// Prefix for logging
func broadcast_domain(s *service.Service) string {
	return s.Name + "_broadcast"
}

// Starts a new broadcaster in the background
// The first parameter is the way to publish the messages.
// The second parameter starts the message
func New(s *service.Service, logger log.Logger) (*Broadcast, error) {
	child := app_log.Child(logger, "broadcast")
	child.SetReportCaller(false)

	broadcast := Broadcast{
		service: s,
		In:      make(chan message.Broadcast),
		logger:  child,
	}

	return &broadcast, nil
}

// We set the whitelisted accounts that has access to this controller
func AddWhitelistedAccounts(s *service.Service, accounts account.Accounts) {
	zmq.AuthCurveAdd(broadcast_domain(s), accounts.BroadcastPublicKeys()...)
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

// Run a new broadcaster
//
// It assumes that the another package is starting an authentication layer of zmq:
// ZAP.
//
// If some error is encountered, then this package panics
func (b *Broadcast) Run() {
	var mu sync.Mutex

	// Socket to talk to clients
	broadcast_socket, err := zmq.NewSocket(zmq.PUB)
	if err != nil {
		b.logger.Fatal("zmq.NewSocket: %w", err)
	}
	b.socket = broadcast_socket

	err = b.socket.Bind(b.service.BroadcastUrl())
	if err != nil {
		b.logger.Fatal("could not listen to publisher", "broadcast_url", b.service.BroadcastUrl(), "message", err)
	}

	sock, err := zmq.NewSocket(zmq.PULL)
	if err != nil {
		b.logger.Fatal("could not create pull socket", "message", err)
	}

	url := "inproc://broadcast_" + b.service.Name
	if err := sock.Connect(url); err != nil {
		b.logger.Fatal("socket binding to %s: %w", url, err)
	}

	b.logger.Info("waiting for new messages...", "url", url)

	for {
		// Wait for reply.
		msgs, _ := sock.RecvMessage(0)
		broadcast, _ := message.ParseBroadcast(msgs)
		b.logger.Info("broadcast a new message", "topic", broadcast.Topic)

		mu.Lock()
		_, err = b.socket.SendMessage(broadcast.Topic, broadcast.ToBytes())
		mu.Unlock()
		if err != nil {
			log.Fatal("socket error to send message", "message", err)
		}
	}
}
