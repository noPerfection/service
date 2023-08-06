package message

import (
	"fmt"
	"github.com/ahmetson/service-lib/log"
	"testing"

	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing orchestra
type TestRequestSuite struct {
	suite.Suite
	ok     Request
	logger *log.Logger
}

// Make sure that Account is set to five
// before each test
func (suite *TestRequestSuite) SetupTest() {
	logger, _ := log.New("test request", false)
	request := Request{
		Command:    "some_command",
		Parameters: key_value.Empty(),
	}
	request.SetUuid()
	request.AddRequestStack("service_1", "name_1", "instance_1")
	request.AddRequestStack("service_2", "name_2", "instance_2")

	suite.logger = logger
	suite.ok = request

	suite.logger.Info("request stack", request)
}

// All methods that begin with "Test" are run as tests within a
// suite.
func (suite *TestRequestSuite) TestIsOk() {
	suite.Empty(suite.ok.GetPublicKey())
}

func (suite *TestRequestSuite) TestToBytes() {
	trace := fmt.Sprintf(`[{"command":"some_command","request_time":%d,"server_instance":"instance_1","server_name":"name_1","service_url":"service_1"},{"command":"some_command","request_time":%d,"server_instance":"instance_2","server_name":"name_2","service_url":"service_2"}],"uuid":"%s"`,
		suite.ok.Trace[0].RequestTime, suite.ok.Trace[1].RequestTime, suite.ok.Uuid)
	okString := fmt.Sprintf(`{"command":"some_command","parameters":{},"trace":%s}`, trace)

	okBytes, err := suite.ok.Bytes()
	suite.NoError(err)

	suite.EqualValues(okString, string(okBytes))

	// The Parameters as a nil should fail
	request := Request{}
	_, err = request.Bytes()
	suite.Error(err)

	// The Failure request can not have an empty message
	request = Request{
		Command: "command",
	}
	_, err = request.Bytes()
	suite.Error(err)

	// The Failure request can not have an empty message
	request = Request{
		Parameters: key_value.Empty(),
	}
	_, err = request.Bytes()
	suite.Error(err)
}

func (suite *TestRequestSuite) TestParsing() {
	okString, _ := suite.ok.Bytes()

	ok, err := ParseRequest([]string{string(okString)})
	suite.Require().NoError(err)

	suite.EqualValues(suite.ok, ok)

	// Parsing a request with the nil values should fail
	invalidReply := `{"command":"","parameters":null}`
	_, err = ParseRequest([]string{invalidReply})
	suite.Error(err)

	// Parsing should fail for missing keys
	invalidReply = `{}`
	_, err = ParseRequest([]string{invalidReply})
	suite.Error(err)

	// Parsing the json with additional field should be
	// successful, but skip the other parameters
	invalidReply = `{"command":"is here","parameters":{},"status":"OK", "sig": ""}`
	_, err = ParseRequest([]string{invalidReply})
	suite.NoError(err)

	// Parsing the request with the missing field should fail
	invalidReply = `{"parameters":{}}`
	_, err = ParseRequest([]string{invalidReply})
	suite.Error(err)

	// Parsing the request with the missing field should fail
	invalidReply = `{"command":"command"}`
	_, err = ParseRequest([]string{invalidReply})
	suite.Error(err)

	// Request parameters are case-insensitive
	// Not way to turn off
	// https://golang.org/pkg/encoding/json/#Unmarshal
	invalidReply = `{"Command":"command","parameters":{}}`
	_, err = ParseRequest([]string{invalidReply})
	suite.NoError(err)

	// Request parsing with the right parameters should succeed
	invalidReply = `{"command":"command","parameters":{}}`
	_, err = ParseRequest([]string{invalidReply})
	suite.NoError(err)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestRequest(t *testing.T) {
	suite.Run(t, new(TestRequestSuite))
}
