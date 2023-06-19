package message

import (
	"testing"

	"github.com/Seascape-Foundation/sds-common-lib/data_type/key_value"
	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestRequestSuite struct {
	suite.Suite
	ok Request
}

// Make sure that Account is set to five
// before each test
func (suite *TestRequestSuite) SetupTest() {
	request := Request{
		Command:    "some_command",
		Parameters: key_value.Empty(),
	}
	suite.ok = request
}

// All methods that begin with "Test" are run as tests within a
// suite.
func (suite *TestRequestSuite) TestIsOk() {
	suite.Empty(suite.ok.GetPublicKey())
}

func (suite *TestRequestSuite) TestToBytes() {
	ok_string := `{"command":"some_command","parameters":{}}`

	ok_bytes, err := suite.ok.ToBytes()
	suite.NoError(err)

	suite.EqualValues(ok_string, string(ok_bytes))

	// The Parameters as a nil should fail
	request := Request{}
	_, err = request.ToBytes()
	suite.Error(err)

	// The Failure request can not have an empty message
	request = Request{
		Command: "command",
	}
	_, err = request.ToBytes()
	suite.Error(err)

	// The Failure request can not have an empty message
	request = Request{
		Parameters: key_value.Empty(),
	}
	_, err = request.ToBytes()
	suite.Error(err)
}

func (suite *TestRequestSuite) TestParsing() {
	ok_string, _ := suite.ok.ToBytes()

	ok, err := ParseRequest([]string{string(ok_string)})
	suite.Require().NoError(err)

	suite.EqualValues(suite.ok, ok)

	// Parsing a request with the nil values should fail
	invalid_reply := `{"command":"","parameters":null}`
	_, err = ParseRequest([]string{invalid_reply})
	suite.Error(err)

	// Parsing should fail for missing keys
	invalid_reply = `{}`
	_, err = ParseRequest([]string{invalid_reply})
	suite.Error(err)

	// Parsing the json with additional field should be
	// successful, but skip the other parameters
	invalid_reply = `{"command":"is here","parameters":{},"status":"OK", "sig": ""}`
	_, err = ParseRequest([]string{invalid_reply})
	suite.NoError(err)

	// Parsing the request with the missing field should fail
	invalid_reply = `{"parameters":{}}`
	_, err = ParseRequest([]string{invalid_reply})
	suite.Error(err)

	// Parsing the request with the missing field should fail
	invalid_reply = `{"command":"command"}`
	_, err = ParseRequest([]string{invalid_reply})
	suite.Error(err)

	// Request parameters are case insensitive
	// Not way to turn off
	// https://golang.org/pkg/encoding/json/#Unmarshal
	invalid_reply = `{"Command":"command","parameters":{}}`
	_, err = ParseRequest([]string{invalid_reply})
	suite.NoError(err)

	// Request parsing with the right parameters should succeed
	invalid_reply = `{"command":"command","parameters":{}}`
	_, err = ParseRequest([]string{invalid_reply})
	suite.NoError(err)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestRequest(t *testing.T) {
	suite.Run(t, new(TestRequestSuite))
}
