package message

import (
	"encoding/hex"
	"testing"

	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestSmRequestSuite struct {
	suite.Suite
	ok           SmartcontractDeveloperRequest
	sample_nonce uint64
}

// Make sure that Account is set to five
// before each test
func (suite *TestSmRequestSuite) SetupTest() {
	suite.sample_nonce = uint64(12312)
	request := Request{
		Command: "get_data",
		Parameters: key_value.Empty().
			Set("_address", "0xdead").
			Set("_nonce_timestamp", suite.sample_nonce).
			Set("_signature", "0xdead").
			// the "get_data" command parameters
			Set("data_id", uint64(1)),
	}

	sm_request, err := ToSmartcontractDeveloperRequest(request)
	suite.NoError(err)
	_, err = sm_request.Request.Parameters.GetString("_address")
	suite.Error(err)
	_, err = sm_request.Request.Parameters.GetString("_signature")
	suite.Error(err)
	_, err = sm_request.Request.Parameters.GetUint64("_nonce_timestamp")
	suite.Error(err)

	// The command parameters should be kept
	_, err = sm_request.Request.Parameters.GetUint64("data_id")
	suite.NoError(err)

	suite.EqualValues(sm_request.NonceTimestamp, suite.sample_nonce)
	suite.EqualValues(sm_request.Signature, "0xdead")
	suite.EqualValues(sm_request.Address, "0xdead")
	suite.Equal(sm_request.Request.Command, "get_data")

	suite.ok = sm_request
}

func (suite *TestSmRequestSuite) TestParsing() {
	// todo
	// check that request's sm developer parmaters
	// deleted after its been converted into sm developer
	// request.

	// Missing _address parameter should fail
	request := Request{
		Command: "get_data",
		Parameters: key_value.Empty().
			Set("_nonce_timestamp", suite.sample_nonce).
			Set("_signature", "0xdead").
			// the "get_data" command parameters
			Set("data_id", 1),
	}
	_, err := ToSmartcontractDeveloperRequest(request)
	suite.Require().Error(err)

	// Empty _address parameter should fail
	request = Request{
		Command: "get_data",
		Parameters: key_value.Empty().
			Set("_address", "").
			Set("_nonce_timestamp", suite.sample_nonce).
			Set("_signature", "0xdead").
			// the "get_data" command parameters
			Set("data_id", 1),
	}
	_, err = ToSmartcontractDeveloperRequest(request)
	suite.Require().Error(err)

	// Invalid request (in this case missing command)
	// Should return an error
	request = Request{
		Parameters: key_value.Empty().
			Set("_nonce_timestamp", suite.sample_nonce).
			Set("_signature", "0xdead").
			// the "get_data" command parameters
			Set("data_id", 1),
	}
	_, err = ToSmartcontractDeveloperRequest(request)
	suite.Require().Error(err)

	// Invalid request (in this an empty command)
	// Should return an error
	request = Request{
		Command: "",
		Parameters: key_value.Empty().
			Set("_address", "0xdead").
			Set("_nonce_timestamp", suite.sample_nonce).
			Set("_signature", "0xdead").
			// the "get_data" command parameters
			Set("data_id", 1),
	}
	_, err = ToSmartcontractDeveloperRequest(request)
	suite.Require().Error(err)

	// Invalid request (in this missing parameters)
	// Should return an error
	request = Request{
		Command:    "get_data",
		Parameters: nil,
	}
	_, err = ToSmartcontractDeveloperRequest(request)
	suite.Require().Error(err)

	// 0 _nonce_timestamp parameter should fail
	request = Request{
		Command: "get_data",
		Parameters: key_value.Empty().
			Set("_address", "0xdead").
			Set("_nonce_timestamp", uint64(0)).
			Set("_signature", "0xdead").
			// the "get_data" command parameters
			Set("data_id", 1),
	}
	_, err = ToSmartcontractDeveloperRequest(request)
	suite.Require().Error(err)

	// Invalid request (in this case missing _nonce_timestmap)
	// Should return an error
	request = Request{
		Command: "get_data",
		Parameters: key_value.Empty().
			Set("_address", "0xdead").
			Set("_signature", "0xdead").
			// the "get_data" command parameters
			Set("data_id", 1),
	}
	_, err = ToSmartcontractDeveloperRequest(request)
	suite.Require().Error(err)

	// missing _signature should fail
	request = Request{
		Command: "get_data",
		Parameters: key_value.Empty().
			Set("_address", "0xdead").
			Set("_nonce_timestamp", suite.sample_nonce).
			// the "get_data" command parameters
			Set("data_id", 1),
	}
	_, err = ToSmartcontractDeveloperRequest(request)
	suite.Require().Error(err)

	// empty _signature should fail
	// Should return an error
	request = Request{
		Command: "get_data",
		Parameters: key_value.Empty().
			Set("_address", "0xdead").
			Set("_nonce_timestamp", suite.sample_nonce).
			Set("_signature", "").
			// the "get_data" command parameters
			Set("data_id", 1),
	}
	_, err = ToSmartcontractDeveloperRequest(request)
	suite.Require().Error(err)

	// empty _address should have a 0x prefix
	// Should return an error
	request = Request{
		Command: "get_data",
		Parameters: key_value.Empty().
			Set("_address", "dead").
			Set("_nonce_timestamp", suite.sample_nonce).
			Set("_signature", "0xdead").
			// the "get_data" command parameters
			Set("data_id", 1),
	}
	_, err = ToSmartcontractDeveloperRequest(request)
	suite.Require().Error(err)

	// empty _address should have a 0x prefix is case insentive
	// Should return an error
	request = Request{
		Command: "get_data",
		Parameters: key_value.Empty().
			Set("_address", "0Xdead").
			Set("_nonce_timestamp", suite.sample_nonce).
			Set("_signature", "0xdead").
			// the "get_data" command parameters
			Set("data_id", 1),
	}
	_, err = ToSmartcontractDeveloperRequest(request)
	suite.Require().NoError(err)

	// empty _signature should have a 0x prefix
	// Should return an error
	request = Request{
		Command: "get_data",
		Parameters: key_value.Empty().
			Set("_address", "0xdead").
			Set("_nonce_timestamp", suite.sample_nonce).
			Set("_signature", "dead").
			// the "get_data" command parameters
			Set("data_id", 1),
	}
	_, err = ToSmartcontractDeveloperRequest(request)
	suite.Require().Error(err)

	// empty _address should have a one value after prefix atleast
	// Should return an error
	request = Request{
		Command: "get_data",
		Parameters: key_value.Empty().
			Set("_address", "0x").
			Set("_nonce_timestamp", suite.sample_nonce).
			Set("_signature", "0xdead").
			// the "get_data" command parameters
			Set("data_id", 1),
	}
	_, err = ToSmartcontractDeveloperRequest(request)
	suite.Require().Error(err)

	suite.ok.message_hash()
}

// Prepare the request's message hash
// Then this message hash along with the signature is used by
// app/account.SmartcontractDeveloper
// to validate the address.
func (suite *TestSmRequestSuite) TestHashing() {
	request := []byte(`{"command":"get_data","parameters":{"_address":"0xdead","_nonce_timestamp":12312,"data_id":1}}`)

	// Use the request string in the link
	// https://emn178.github.io/online-tools/keccak_256.html
	expected_hash, _ := hex.DecodeString("a71cd8b2a2004b3d41ce9c9f33c405f663858d963c6dc4c7fe6a22a7d5c18451")
	calculated_hash := crypto.Keccak256Hash(request)

	hash_bytes, err := suite.ok.message_hash()
	suite.Require().NoError(err)
	suite.Require().Equal(expected_hash, hash_bytes)
	suite.Require().Equal(calculated_hash.Bytes(), hash_bytes)

	prefix := []byte("\x19Ethereum Signed Message:\n32")
	full_message := append(prefix, hash_bytes...)
	calculated_digest_hash := crypto.Keccak256Hash(full_message)

	// Use the prefix in the hex format:
	// full_message := append(hex.EncodeToString(prefix), hash_bytes...)
	// full_hash := hex.EncodeToString(full_message)
	//
	// Then pass the full_hash to
	// https://emn178.github.io/online-tools/keccak_256.html
	expected_digest_hash, _ := hex.DecodeString("337dc5266f47b40d69ff6df7a9ca09513aaf81bd951ed1dd5fcb71f8432e2bee")

	digested_hash, err := suite.ok.DigestedMessage()
	suite.Require().NoError(err)
	suite.Require().Equal(expected_digest_hash, digested_hash)
	suite.Require().Equal(calculated_digest_hash.Bytes(), digested_hash)

}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestSmDeveloperRequest(t *testing.T) {
	suite.Run(t, new(TestSmRequestSuite))
}
