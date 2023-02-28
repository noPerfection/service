// Broadcast package creates a publishing socket
// Use this package in a goroutine.
package broadcast

import (
	"fmt"
	"log"
	"sync"

	"github.com/blocklords/gosds/app/account"
	"github.com/blocklords/gosds/app/service"

	"github.com/blocklords/gosds/app/remote/message"

	zmq "github.com/pebbe/zmq4"
)

// Broadcast
type Broadcast struct {
	service *service.Service
	socket  *zmq.Socket
	In      chan message.Broadcast
}

// Prefix for logging
func broadcast_domain(s *service.Service) string {
	return s.Name + "_broadcast"
}

// Starts a new broadcaster in the background
// The first parameter is the way to publish the messages.
// The second parameter starts the message
func New(s *service.Service) (*Broadcast, error) {
	// Socket to talk to clients
	socket, err := zmq.NewSocket(zmq.PUB)
	if err != nil {
		return nil, fmt.Errorf("zmq.NewSocket: %w", err)
	}

	broadcast := Broadcast{
		socket:  socket,
		service: s,
		In:      make(chan message.Broadcast),
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
	err := c.socket.ServerAuthCurve(broadcast_domain(c.service), c.service.BroadcastSecretKey)
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

	mu.Lock()
	err := b.socket.Bind("tcp://*:" + b.service.BroadcastPort())
	if err != nil {
		log.Fatalf("could not listen to publisher: %v", err)
	}
	mu.Unlock()

	for {
		broadcast := <-b.In

		mu.Lock()
		_, err = b.socket.SendMessage(broadcast.Topic, broadcast.ToBytes())
		mu.Unlock()
		if err != nil {
			log.Fatal(err)
		}
	}
}
