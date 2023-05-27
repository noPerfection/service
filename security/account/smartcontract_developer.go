// Handles the user's authentication
package account

import (
	"crypto/ecdsa"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"

	"github.com/blocklords/sds/service/communication/message"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/ecies"
)

// Different blockchains use different
// Public/Private key pair algorithms.
//
// SDS as the blockchain agnostic tries to support
// any blockchain's wallet
type CurveType = uint8

// ECDSA CurveType is the public/private key algorithm
// used in Ethereum and other EVM based blockchains.
const ECDSA CurveType = 1

// Smartcontract developer uses Ecdsapublic key
type SmartcontractDeveloper struct {
	Address         string
	AccountType     CurveType         // The cryptographic algorithm key
	EcdsaPublicKey  *ecdsa.PublicKey  // If the account type is ECDSA, then this one will keep the pub key
	EcdsaPrivateKey *ecdsa.PrivateKey //
}

// Creates a new SmartcontractDeveloper with a public key but without private key
func NewEcdsaPublicKey(pub_key *ecdsa.PublicKey) *SmartcontractDeveloper {
	return &SmartcontractDeveloper{
		Address:         PublicKeyToAddress(pub_key),
		AccountType:     ECDSA,
		EcdsaPublicKey:  pub_key,
		EcdsaPrivateKey: nil,
	}
}

// Creates a new SmartcontractDeveloper with a private key
func NewEcdsaPrivateKey(private_key *ecdsa.PrivateKey) *SmartcontractDeveloper {
	return &SmartcontractDeveloper{
		Address:         PublicKeyToAddress(&private_key.PublicKey),
		AccountType:     ECDSA,
		EcdsaPublicKey:  &private_key.PublicKey,
		EcdsaPrivateKey: private_key,
	}
}

// PublicKeyToAddress is a utility function for SmartcontractDeveloper accounts.
// It derives the address from the public key in ECDSA algorithm.
func PublicKeyToAddress(public_key *ecdsa.PublicKey) string {
	return crypto.PubkeyToAddress(*public_key).Hex()
}

// Get the account who did the request.
// Account is verified first using the signature parameter of the request.
// If the signature is not a valid, then returns an error.
//
// For now it supports ECDSA addresses only. Therefore verification automatically assumes that address
// is for the ethereum network.
func VerifySignature(request *message.SmartcontractDeveloperRequest) error {
	_, err := GetPublicKeyAccount(request)
	if err != nil {
		return fmt.Errorf("GetPublicKeyAccount: %w", err)
	}

	return nil
}

// Returns the account with the public key.
// The public key derived from the signature.
//
// Call this function after VerifySignature()
// Because it doesn't check for errors
func GetPublicKeyAccount(request *message.SmartcontractDeveloperRequest) (*SmartcontractDeveloper, error) {
	// without 0x prefix
	signature, err := hexutil.Decode(request.Signature)
	if err != nil {
		return nil, fmt.Errorf("hexutil.Decode: %w", err)
	}
	digested_hash, err := request.DigestedMessage()
	if err != nil {
		return nil, fmt.Errorf("request.DigestMessage: %w", err)
	}

	if len(signature) != 65 {
		return nil, fmt.Errorf("the ECDSA signature length is invalid. It should be 64 bytes long. Signature length: %d", len(signature))
	}
	if signature[64] != 27 && signature[64] != 28 {
		return nil, errors.New("invalid Ethereum signature (V is not 27 or 28)")
	}
	signature[64] -= 27 // Transform yellow paper V from 27/28 to 0/1

	ecdsa_public_key, err := crypto.SigToPub(digested_hash, signature)
	if err != nil {
		return nil, fmt.Errorf("crypto.SigToPub for %v hash and %s signature: %w", digested_hash, string(signature), err)
	}

	address := PublicKeyToAddress(ecdsa_public_key)
	if !strings.EqualFold(address, request.Address) {
		return nil, fmt.Errorf("derived address %s mismatch address in the message %s", address, request.Address)
	}

	return NewEcdsaPublicKey(ecdsa_public_key), nil
}

// Encrypt the given data with a public key
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
	if err != nil {
		return []byte{}, fmt.Errorf("ecies.encrypt by public key %v: %w", curve_pb, err)
	}

	return cipher_text, nil
}

// Decrypt the given cipher text to the plain text by SmartcontractDeveloper account.
//
// The smartcontract developer account must be with the private key.
func (account *SmartcontractDeveloper) Decrypt(cipher_text []byte) ([]byte, error) {
	if account.AccountType != ECDSA {
		return []byte{}, errors.New("only ECDSA is supported")
	}

	if account.EcdsaPrivateKey == nil {
		return []byte{}, errors.New("the account has no private key")
	}

	curve_secret_key := ecies.ImportECDSA(account.EcdsaPrivateKey)
	plain_text, err := curve_secret_key.Decrypt(cipher_text, nil, nil)

	if err != nil {
		return []byte{}, fmt.Errorf("ecies.decrypt cipher text %v: %w", cipher_text, err)
	}

	return plain_text, err
}
