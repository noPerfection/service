package event

import (
	"testing"

	"github.com/blocklords/sds/common/blockchain"
	"github.com/stretchr/testify/suite"

	"github.com/ethereum/go-ethereum/common"
	eth_types "github.com/ethereum/go-ethereum/core/types"
)

// We won't test the requests.
// The requests are tested in the controllers
// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestLogSuite struct {
	suite.Suite
	network_id string
	timestamp  blockchain.Timestamp
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
func (suite *TestLogSuite) SetupTest() {
	suite.network_id = "1"
	timestamp, err := blockchain.NewTimestamp(uint64(2))
	suite.Require().NoError(err)
	suite.timestamp = timestamp
}

func (suite *TestLogSuite) TestNew() {
	// can't pass nil as ethereum log.
	// it should fail
	_, err := NewSpaghettiLog(suite.network_id, suite.timestamp, nil)
	suite.Require().Error(err)

	// valid log
	eth_log := eth_types.Log{
		Address:     common.HexToAddress("0xC6EF8A96F20d50E347eD9a1C84142D02b1EFedc0"),
		BlockNumber: 2,
		TxHash:      common.HexToHash("0xC6EF8A96F20d50E347eD9a1C84142D02b1EFedc0"),
		TxIndex:     0,
		Data:        []byte{01, 02, 03, 04},
	}
	raw_log, err := NewSpaghettiLog(suite.network_id, suite.timestamp, &eth_log)

	// we don't insert transaction sender
	suite.Empty(raw_log.Transaction.From)

	suite.Require().NoError(err)
	suite.Equal("01020304", raw_log.Data)
	// just to avoid error in Validate()
	raw_log.Transaction.From = "0x123"
	err = raw_log.Transaction.Validate()
	suite.Require().NoError(err)
	suite.Equal("0xC6EF8A96F20d50E347eD9a1C84142D02b1EFedc0", raw_log.Transaction.SmartcontractKey.Address)
	suite.Equal(suite.network_id, raw_log.Transaction.SmartcontractKey.NetworkId)
	suite.Equal("0x000000000000000000000000c6ef8a96f20d50e347ed9a1c84142d02b1efedc0", raw_log.Transaction.TransactionKey.Id)
	suite.Equal(uint64(2), raw_log.Transaction.BlockHeader.Number.Value())

}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestLog(t *testing.T) {
	suite.Run(t, new(TestLogSuite))
}
