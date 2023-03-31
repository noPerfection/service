package message

import (
	"testing"

	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestBroadcastSuite struct {
	suite.Suite
	fail_broadcast Broadcast
	ok_broadcast   Broadcast
}

// Make sure that Account is set to five
// before each test
func (suite *TestBroadcastSuite) SetupTest() {
	topic := "random_topic"
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
	suite.ok_broadcast = NewBroadcast(topic, reply)
	suite.fail_broadcast = NewBroadcast(topic, fail_reply)
}

// All methods that begin with "Test" are run as tests within a
// suite.
func (suite *TestBroadcastSuite) TestIsOk() {
	suite.True(suite.ok_broadcast.IsOK())
	suite.False(suite.fail_broadcast.IsOK())
}

func (suite *TestBroadcastSuite) TestToBytes() {
	ok_string := `{"reply":{"message":"","parameters":{},"status":"OK"},"topic":"random_topic"}`
	fail_string := `{"reply":{"message":"failed for testing purpose","parameters":{},"status":"fail"},"topic":"random_topic"}`

	fail_bytes := suite.fail_broadcast.ToBytes()
	ok_bytes := suite.ok_broadcast.ToBytes()

	suite.EqualValues(ok_string, string(ok_bytes))
	suite.EqualValues(fail_string, string(fail_bytes))
}

func (suite *TestBroadcastSuite) TestParsing() {
	fail_string := string(suite.fail_broadcast.ToBytes())
	ok_string := string(suite.ok_broadcast.ToBytes())

	ok_broadcast, err := ParseBroadcast([]string{suite.ok_broadcast.Topic, ok_string})
	suite.Require().NoError(err)
	fail_broadcast, err := ParseBroadcast([]string{suite.fail_broadcast.Topic, fail_string})
	suite.Require().NoError(err)

	suite.EqualValues(suite.ok_broadcast, ok_broadcast)
	suite.EqualValues(suite.fail_broadcast, fail_broadcast)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestBroadcast(t *testing.T) {
	suite.Run(t, new(TestBroadcastSuite))
}
