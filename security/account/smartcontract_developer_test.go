package account

import (
	"encoding/hex"
	"testing"

	"crypto/ecdsa"

	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/service/communication/message"
	"github.com/stretchr/testify/suite"

	"github.com/ethereum/go-ethereum/crypto"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestSmDeveloperSuite struct {
	suite.Suite
	EcdsaPrivateKey *SmartcontractDeveloper
	EcdsaPublicKey  *SmartcontractDeveloper
	address         string
	private_key     *ecdsa.PrivateKey
	public_key      *ecdsa.PublicKey
	request_message message.SmartcontractDeveloperRequest
}

// Make sure that Account is set to five
// before each test
//
// Derive the public key using etherjs:
// http://jsfiddle.net/ztj5ywdb/11/
func (suite *TestSmDeveloperSuite) SetupTest() {
	private_key_string := "fb5f7c3e4c2d2668e984b0e8815fa0fa98b3ddb498c87478cee40b736d5efc7c"
	public_key_string := "04a5da7f7acd449b8d30ee605019413a69af2b4e7d28d7226e42ba9bf162afaa673b2c3f3ed63b0a18d44658cdb9ac2e05002e9f8b63941ca596ebf5991e18607d"
	address := "0x5bDed8f6BdAE766C361EDaE25c5DC966BCaF8f43"

	private_key, err := crypto.HexToECDSA(private_key_string)
	suite.Empty(err)

	public_key_bytes, err := hex.DecodeString(public_key_string)
	suite.Empty(err)
	public_key, err := crypto.UnmarshalPubkey(public_key_bytes)
	suite.Empty(err)

	suite.address = address
	suite.private_key = private_key
	suite.public_key = public_key

	request := message.SmartcontractDeveloperRequest{
		Address:        address,
		NonceTimestamp: 1,
		Request: message.Request{
			Command: "command",
			Parameters: key_value.Empty().
				Set("bool_param", true).
				Set("string_param", "hello_world").
				Set("number_param", uint64(64)),
		},
	}
	hash, err := request.DigestedMessage()
	suite.Nil(err, "failed to create request hash", err)
	signature, err := crypto.Sign(hash, suite.private_key)
	suite.Nil(err, "failed to generate signature", err)
	signature[64] += 27
	request.Signature = "0x" + hex.EncodeToString(signature)

	suite.EcdsaPublicKey = NewEcdsaPublicKey(suite.public_key)
	suite.EcdsaPrivateKey = NewEcdsaPrivateKey(suite.private_key)
	suite.request_message = request
}

// All methods that begin with "Test" are run as tests within a
// suite.
func (suite *TestSmDeveloperSuite) TestAddresses() {
	suite.Equal(suite.EcdsaPublicKey.Address, suite.address)
	suite.Equal(suite.EcdsaPrivateKey.Address, suite.address)
}

func (suite *TestSmDeveloperSuite) TestPublicKeys() {
	suite.Equal(suite.EcdsaPublicKey.EcdsaPublicKey, suite.public_key)
	suite.Equal(suite.EcdsaPrivateKey.EcdsaPublicKey, suite.public_key)
}

func (suite *TestSmDeveloperSuite) TestPrivateKeys() {
	suite.NotEmpty(suite.EcdsaPrivateKey.EcdsaPrivateKey)
	suite.Empty(suite.EcdsaPublicKey.EcdsaPrivateKey)
}

func (suite *TestSmDeveloperSuite) TestAccountType() {
	suite.Equal(suite.EcdsaPrivateKey.AccountType, ECDSA)
	suite.Equal(suite.EcdsaPublicKey.AccountType, ECDSA)
}

func (suite *TestSmDeveloperSuite) TestSignatureVerification() {
	err := VerifySignature(&suite.request_message)
	suite.Nil(err, "SignatureVerification failed", err)
}

// Returns the SmartcontractDeveloper from
// the message.
//
// This function also runs the checks in the signature.
func (suite *TestSmDeveloperSuite) TestMessageToSmDeveloper() {
	err := VerifySignature(&suite.request_message)
	suite.Nil(err, "SignatureVerification failed", err)
}

// First the account with public key encrypts
// Then the account with the private key decrypts
func (suite *TestSmDeveloperSuite) TestEncryption() {
	plain_text := []byte("hello_suite")

	cipher_text, err := suite.EcdsaPublicKey.Encrypt(plain_text)
	suite.Nil(err, "SmDeveloper.Encrypt failed", err)

	decrypted, err := suite.EcdsaPrivateKey.Decrypt(cipher_text)
	suite.Nil(err, "SmDeveloper.Decrypt failed", err)
	suite.Equal(plain_text, decrypted)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestSmDeveloper(t *testing.T) {
	suite.Run(t, new(TestSmDeveloperSuite))
}
