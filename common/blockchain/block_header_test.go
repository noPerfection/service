package blockchain

import (
	"testing"

	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/stretchr/testify/suite"
)

// We won't test the requests.
// The requests are tested in the controllers
// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestBlockHeaderSuite struct {
	suite.Suite
	header BlockHeader
}

// Test setup (inproc, tcp and sub)
//	Along with the reconnect
// Test Requests (router, remote)
// Test the timeouts
// Test close (attempt to request)

// Todo test inprocess and external types of controllers
// Todo test the business of the controller
// Make sure that Account is set to five
// before each test
func (suite *TestBlockHeaderSuite) SetupTest() {
	uint_number := uint64(10)
	uint_timestamp := uint64(123)
	uint_map := key_value.Empty().
		Set("block_number", uint_number).
		Set("block_timestamp", uint_timestamp)

	header := NewHeader(uint_number, uint_timestamp)
	map_header, err := NewHeaderFromKeyValueParameter(uint_map)
	suite.Require().NoError(err)
	number := NewNumber(uint_number)
	map_number, err := NewNumberFromKeyValueParameter(uint_map)
	suite.Require().NoError(err)
	timestamp := NewTimestamp(uint_timestamp)
	map_timestamp, err := NewTimestampFromKeyValueParameter(uint_map)
	suite.Require().NoError(err)

	suite.Require().Equal(header, map_header)
	suite.Require().Equal(header.Number.Value(), uint_number)
	suite.Require().Equal(header.Number, number)
	suite.Require().Equal(header.Number, map_number)
	suite.Require().Equal(header.Timestamp.Value(), uint_timestamp)
	suite.Require().Equal(header.Timestamp, timestamp)
	suite.Require().Equal(header.Timestamp, map_timestamp)

	suite.header = header

	// Missing both parameters
	empty_map := key_value.Empty()
	_, err = NewHeaderFromKeyValueParameter(empty_map)
	suite.Require().Error(err)

	// Missing timestamp, should fail
	no_timestamp_map := key_value.Empty().
		Set("block_number", uint_number)
	_, err = NewHeaderFromKeyValueParameter(no_timestamp_map)
	suite.Require().Error(err)

	// Missing timestamp, should fail
	no_number_map := key_value.Empty().
		Set("block_timestamp", uint_timestamp)
	_, err = NewHeaderFromKeyValueParameter(no_number_map)
	suite.Require().Error(err)
}

func (suite *TestBlockHeaderSuite) TestValueChange() {
	suite.Equal(NewNumber(11), suite.header.Number.Increment())
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestBlockHeader(t *testing.T) {
	suite.Run(t, new(TestBlockHeaderSuite))
}
