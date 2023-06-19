package message

import (
	"testing"

	"github.com/Seascape-Foundation/sds-service-lib/common/data_type/key_value"
	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestReplySuite struct {
	suite.Suite
	fail Reply
	ok   Reply
}

// Make sure that Account is set to five
// before each test
func (suite *TestReplySuite) SetupTest() {
	reply := Reply{
		Status:     OK,
		Message:    "",
		Parameters: key_value.Empty(),
	}
	fail_reply := Reply{
		Status:     FAIL,
		Message:    "failed for testing purpose",
		Parameters: key_value.Empty(),
	}
	suite.ok = reply
	suite.fail = fail_reply
}

// All methods that begin with "Test" are run as tests within a
// suite.
func (suite *TestReplySuite) TestIsOk() {
	suite.True(suite.ok.IsOK())
	suite.False(suite.fail.IsOK())
}

func (suite *TestReplySuite) TestToBytes() {
	ok_string := `{"message":"","parameters":{},"status":"OK"}`
	fail_string := `{"message":"failed for testing purpose","parameters":{},"status":"fail"}`

	fail_bytes, err := suite.fail.ToBytes()
	suite.NoError(err)
	ok_bytes, err := suite.ok.ToBytes()
	suite.NoError(err)

	suite.EqualValues(ok_string, string(ok_bytes))
	suite.EqualValues(fail_string, string(fail_bytes))

	// The Parameters as a nil should fail
	reply := Reply{
		Status:  FAIL,
		Message: "failed for testing purpose",
	}
	_, err = reply.ToBytes()
	suite.Error(err)

	// The Failure reply can not have an empty message
	reply = Reply{
		Status:     FAIL,
		Parameters: key_value.Empty(),
	}
	_, err = reply.ToBytes()
	suite.Error(err)

	// The Failure reply can not have an empty message
	reply = Reply{
		Message:    "",
		Parameters: key_value.Empty(),
	}
	_, err = reply.ToBytes()
	suite.Error(err)
}

func (suite *TestReplySuite) TestParsing() {
	fail_string, _ := suite.fail.ToBytes()
	ok_string, _ := suite.ok.ToBytes()

	ok, err := ParseReply([]string{string(ok_string)})
	suite.Require().NoError(err)
	fail, err := ParseReply([]string{string(fail_string)})
	suite.Require().NoError(err)

	suite.EqualValues(suite.ok, ok)
	suite.EqualValues(suite.fail, fail)

	// Parsing a reply with the nil values should fail
	invalid_reply := `{"message":"","parameters":null,"status":"OK"}`
	_, err = ParseReply([]string{invalid_reply})
	suite.Error(err)

	// Parsing a reply with an invalid error should fail
	invalid_reply = `{"message":"","parameters":{},"status":""}`
	_, err = ParseReply([]string{invalid_reply})
	suite.Error(err)

	// Parsing should fail for missing keys
	invalid_reply = `{}`
	_, err = ParseReply([]string{invalid_reply})
	suite.Error(err)

	// Parsing the json with additional field should be
	// successful, but skip the other parameters
	invalid_reply = `{"message":"","parameters":{},"status":"OK", "sig": ""}`
	_, err = ParseReply([]string{invalid_reply})
	suite.NoError(err)

	// Parsing the failure with an empty message should fail
	invalid_reply = `{"message":"","parameters":{},"status":"fail", "sig": ""}`
	_, err = ParseReply([]string{invalid_reply})
	suite.Error(err)

	// Parsing the reply with the missing field should fail
	invalid_reply = `{"message":"","parameters":{}}`
	_, err = ParseReply([]string{invalid_reply})
	suite.Error(err)

	// Parsing the reply with the missing field should fail
	invalid_reply = `{"message":"","status":"OK"}`
	_, err = ParseReply([]string{invalid_reply})
	suite.Error(err)

	// Parsing the reply with the missing scalar type
	// should be successful, since we will set the default
	// values
	invalid_reply = `{"parameters":{}, "status":"OK"}`
	_, err = ParseReply([]string{invalid_reply})
	suite.NoError(err)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestReply(t *testing.T) {
	suite.Run(t, new(TestReplySuite))
}
