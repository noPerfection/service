package credentials

import (
	"fmt"

	"github.com/blocklords/sds/security/vault"

	zmq "github.com/pebbe/zmq4"
)

type Credentials struct {
	PublicKey   string
	private_key string
}

// Fetch the given credentials from the Vault.
// It fetches the private key from the vault.
// Then gets the public key from it
func NewFromVault(bucket string, key string) (*Credentials, error) {
	private_key, err := vault.GetStringFromVault(bucket, key)
	if err != nil {
		return nil, fmt.Errorf("vault: %w", err)
	}

	pub_key, err := zmq.AuthCurvePublic(private_key)
	if err != nil {
		return nil, fmt.Errorf("zmq.Convert Secret to Pub: %w", err)
	}

	return &Credentials{
		private_key: private_key,
		PublicKey:   pub_key,
	}, nil
}

// Returns the credentials but with public key only
func New(public_key string) *Credentials {
	return &Credentials{
		PublicKey:   public_key,
		private_key: "",
	}
}

// Sets the private key to the socket on a given domain
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

// Sets the authentication to the target machine
// Considering that this is the client
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
