// Handles the user's authentication
package account

import (
	"crypto/ecdsa"
	"crypto/rand"
	"errors"
	"strings"

	"github.com/blocklords/gosds/app/remote/message"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/ecies"
)

const ECDSA uint8 = 1

type SmartcontractDeveloper struct {
	Address         string
	AccountType     uint8             // The cryptographic algorithm key
	EcdsaPublicKey  *ecdsa.PublicKey  // If the account type is ECDSA, then this one will keep the pub key
	EcdsaPrivateKey *ecdsa.PrivateKey //
}

// Creates a new SmartcontractDeveloper with a public key but without private key
func NewEcdsaPublicKey(pub_key *ecdsa.PublicKey) *SmartcontractDeveloper {
	return &SmartcontractDeveloper{
		Address:         crypto.PubkeyToAddress(*pub_key).Hex(),
		AccountType:     ECDSA,
		EcdsaPublicKey:  pub_key,
		EcdsaPrivateKey: nil,
	}
}

// Creates a new SmartcontractDeveloper with a private key
func NewEcdsaPrivateKey(private_key *ecdsa.PrivateKey) *SmartcontractDeveloper {
	return &SmartcontractDeveloper{
		Address:         crypto.PubkeyToAddress(private_key.PublicKey).Hex(),
		AccountType:     ECDSA,
		EcdsaPublicKey:  &private_key.PublicKey,
		EcdsaPrivateKey: private_key,
	}
}

// Get the account who did the request.
// Account is verified first using the signature parameter of the request.
// If the signature is not a valid, then returns an error.
//
// For now it supports ECDSA addresses only. Therefore verification automatically assumes that address
// is for the ethereum network.
func NewSmartcontractDeveloper(request *message.SmartcontractDeveloperRequest) (*SmartcontractDeveloper, error) {
	// without 0x prefix
	signature, err := hexutil.Decode(request.Signature)
	if err != nil {
		return nil, err
	}
	digested_hash := request.DigestedMessage()

	if len(signature) != 65 {
		return nil, errors.New("the ECDSA signature length is invalid. It should be 64 bytes long. Signature length: ")
	}
	if signature[64] != 27 && signature[64] != 28 {
		return nil, errors.New("invalid Ethereum signature (V is not 27 or 28)")
	}
	signature[64] -= 27 // Transform yellow paper V from 27/28 to 0/1

	ecdsa_public_key, err := crypto.SigToPub(digested_hash, signature)
	if err != nil {
		return nil, err
	}

	address := crypto.PubkeyToAddress(*ecdsa_public_key).Hex()
	if !strings.EqualFold(address, request.Address) {
		return nil, errors.New("the request 'address' parameter mismatches to the account derived from signature. Account derived from the signature: " + address + "...")
	}

	return NewEcdsaPublicKey(ecdsa_public_key), nil
}

// Encrypts the given data with a public key
// The result could be decrypted by the private key
//
// If the account has a private key, then the public key derived from it would be used
func (account *SmartcontractDeveloper) Encrypt(plain_text []byte) ([]byte, error) {
	if account.AccountType != ECDSA {
		return []byte{}, errors.New("only ECDSA protocol supported")
	}
	if account.EcdsaPrivateKey != nil {
		account.EcdsaPublicKey = &account.EcdsaPrivateKey.PublicKey
	}
	if account.EcdsaPublicKey == nil {
		return []byte{}, errors.New("the account has no public key")
	}
	// We get the public key for from the signature.
	// We also get the account address from public key using crypto.PubkeyToAddress(*ethereum_pb).Hex()
	// this account should be checked against the whitelisted accounts in the database.
	//
	// Encrypt the message with the public key
	curve_pb := ecies.ImportECDSAPublic(account.EcdsaPublicKey)
	cipher_text, err := ecies.Encrypt(rand.Reader, curve_pb, plain_text, nil, nil)

	return cipher_text, err
}

func (account *SmartcontractDeveloper) Decrypt(cipher_text []byte) ([]byte, error) {
	if account.AccountType != ECDSA {
		return []byte{}, errors.New("only ECDSA is supported")
	}

	if account.EcdsaPrivateKey == nil {
		return []byte{}, errors.New("the account has no private key")
	}

	curve_secret_key := ecies.ImportECDSA(account.EcdsaPrivateKey)
	plain_text, err := curve_secret_key.Decrypt(cipher_text, nil, nil)
	return plain_text, err
}
