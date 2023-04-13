// Package auth defines the CURVE public/private key
// used for authentication of socket interaction.
//
// If the sockets are exchanging messages in TCP protocol on the production
// environment, then advised to use this authentication.
//
// For inproc protocol, the authentication is not needed.
package auth

import (
	"fmt"

	zmq "github.com/pebbe/zmq4"
)

// The ZAP curve for authentication
type Credentials struct {
	PublicKey   string
	private_key string
}

// NewPrivateKey returns credentials with public key and private key
func NewPrivateKey(private_key string, public_key string) *Credentials {
	return &Credentials{
		PublicKey:   public_key,
		private_key: private_key,
	}
}

// New credentials with public key only
func New(public_key string) *Credentials {
	return &Credentials{
		PublicKey:   public_key,
		private_key: "",
	}
}

// HasPrivateKey checks whether the Credentials has
// private key or not.
//
// If Credentials was created directly or as New(), then this function will return false.
// If Credentials was created using NewPrivateKey(), then this function will return true.
func (c *Credentials) HasPrivateKey() bool {
	return len(c.private_key) > 0
}

// SetSocketAuthCurve sets the private key to the socket on a given domain.
// Call it for controllers.
func (c *Credentials) SetSocketAuthCurve(socket *zmq.Socket, domain string) error {
	if len(c.private_key) == 0 {
		return fmt.Errorf("no private key")
	}
	err := socket.ServerAuthCurve(domain, c.private_key)
	if err != nil {
		return fmt.Errorf("socket.ServerAuthCurve: %w", err)
	}
	return nil
}

// SetClientAuthCurve sets the authentication of the client.
// Call it for [app/remote.ClientSocket]
//
// The server_public_key is the public key derived from Credentials for the controller socket.
func (c *Credentials) SetClientAuthCurve(socket *zmq.Socket, server_public_key string) error {
	if len(c.private_key) == 0 {
		return fmt.Errorf("no client private key")
	}
	err := socket.ClientAuthCurve(server_public_key, c.PublicKey, c.private_key)
	if err != nil {
		return fmt.Errorf("socket.ClientAuthCurve: %w", err)
	}
	return nil
}
