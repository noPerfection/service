package smartcontract

import (
	"testing"

	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/smartcontract_key"
	"github.com/stretchr/testify/suite"
)

// We won't test the requests.
// The requests are tested in the controllers
// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestSmartcontractSuite struct {
	suite.Suite
	key          smartcontract_key.Key
	block_header blockchain.BlockHeader
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
func (suite *TestSmartcontractSuite) SetupTest() {
	suite.key = smartcontract_key.Key{
		NetworkId: "1",
		Address:   "0xdead",
	}
	suite.block_header, _ = blockchain.NewHeader(uint64(1), uint64(2))
}

func (suite *TestSmartcontractSuite) TestNew() {
	// getting from the key value
	kv := key_value.Empty().
		Set("smartcontract_key", key_value.Empty().
			Set("network_id", "1").
			Set("address", "address")).
		Set("block_header", key_value.Empty().
			Set("block_number", 23).
			Set("block_timestamp", 123))
	_, err := New(kv)
	suite.Require().NoError(err)

	sm := Smartcontract{
		SmartcontractKey: suite.key,
		BlockHeader:      suite.block_header,
	}
	suite.Require().NoError(sm.Validate())

	new_header := blockchain.BlockHeader{
		Number:    blockchain.Number(12),
		Timestamp: blockchain.Timestamp(12),
	}
	sm.SetBlockHeader(new_header)
	suite.Require().NoError(sm.Validate())

	new_header = blockchain.BlockHeader{
		Number:    blockchain.Number(0),
		Timestamp: blockchain.Timestamp(12),
	}
	sm.SetBlockHeader(new_header)
	suite.Require().Error(sm.Validate())
}

func (suite *TestSmartcontractSuite) TestToString() {
	expected_string := `{"block_header":{"block_number":1,"block_timestamp":2},"smartcontract_key":{"address":"0xdead","network_id":"1"}}`
	sm := Smartcontract{
		SmartcontractKey: suite.key,
		BlockHeader:      suite.block_header,
	}
	str, _ := sm.ToString()
	suite.Require().EqualValues(expected_string, str)
}

func TestSmartcontract(t *testing.T) {
	suite.Run(t, new(TestSmartcontractSuite))
}
