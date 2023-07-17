package message

import (
	"testing"

	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestBroadcastSuite struct {
	suite.Suite
	failBroadcast Broadcast
	okBroadcast   Broadcast
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
	failReply := Reply{
		Status:     FAIL,
		Message:    "failed for testing purpose",
		Parameters: key_value.Empty(),
	}
	suite.okBroadcast = NewBroadcast(topic, reply)
	suite.failBroadcast = NewBroadcast(topic, failReply)
}

// All methods that begin with "Test" are run as tests within a
// suite.
func (suite *TestBroadcastSuite) TestIsOk() {
	suite.True(suite.okBroadcast.IsOK())
	suite.False(suite.failBroadcast.IsOK())
}

func (suite *TestBroadcastSuite) TestToBytes() {
	okString := `{"reply":{"message":"","parameters":{},"status":"OK"},"topic":"random_topic"}`
	failString := `{"reply":{"message":"failed for testing purpose","parameters":{},"status":"fail"},"topic":"random_topic"}`

	failBytes := suite.failBroadcast.ToBytes()
	okBytes := suite.okBroadcast.ToBytes()

	suite.EqualValues(okString, string(okBytes))
	suite.EqualValues(failString, string(failBytes))
}

func (suite *TestBroadcastSuite) TestParsing() {
	failString := string(suite.failBroadcast.ToBytes())
	okString := string(suite.okBroadcast.ToBytes())

	okBroadcast, err := ParseBroadcast([]string{suite.okBroadcast.Topic, okString})
	suite.Require().NoError(err)
	failBroadcast, err := ParseBroadcast([]string{suite.failBroadcast.Topic, failString})
	suite.Require().NoError(err)

	suite.EqualValues(suite.okBroadcast, okBroadcast)
	suite.EqualValues(suite.failBroadcast, failBroadcast)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestBroadcast(t *testing.T) {
	suite.Run(t, new(TestBroadcastSuite))
}
