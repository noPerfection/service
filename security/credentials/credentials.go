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
