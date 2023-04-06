package block

import (
	"testing"

	"github.com/blocklords/sds/blockchain/evm/event"
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
type TestBlockSuite struct {
	suite.Suite
	network_id string
	timestamp  blockchain.Timestamp
	block      Block
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
func (suite *TestBlockSuite) SetupTest() {
	suite.network_id = "1"
	timestamp, err := blockchain.NewTimestamp(uint64(2))
	suite.Require().NoError(err)
	suite.timestamp = timestamp

	suite.block = Block{
		NetworkId: "1",
		Header: blockchain.BlockHeader{
			Number:    blockchain.Number(1),
			Timestamp: timestamp,
		},
	}

	// can't pass nil as ethereum log.
	// valid log
	eth_log := eth_types.Log{
		Address:     common.HexToAddress("0xC6EF8A96F20d50E347eD9a1C84142D02b1EFedc0"),
		BlockNumber: 2,
		TxHash:      common.HexToHash("0xC6EF8A96F20d50E347eD9a1C84142D02b1EFedc0"),
		TxIndex:     0,
		Data:        []byte{01, 02, 03, 04},
	}
	raw_log, err := event.NewSpaghettiLog(suite.network_id, suite.timestamp, &eth_log)
	suite.Require().NoError(err)

	eth_log_1 := eth_types.Log{
		Address:     common.HexToAddress("0xFEEF8A96F20d50E347eD9a1C84142D02b1EFedc0"),
		BlockNumber: 2,
		TxHash:      common.HexToHash("0xC6EF8A96F20d50E347eD9a1C84142D02b1EFedc0"),
		TxIndex:     0,
		Data:        []byte{01, 02, 03, 04},
	}
	raw_log_1, err := event.NewSpaghettiLog(suite.network_id, suite.timestamp, &eth_log_1)
	suite.Require().NoError(err)

	eth_log_2 := eth_types.Log{
		Address:     common.HexToAddress("0xFEEF8A96F20d50E347eD9a1C84142D02b1EFedc0"),
		BlockNumber: 2,
		TxHash:      common.HexToHash("0xC6EF8A96F20d50E347eD9a1C84142D02b1EFedc0"),
		TxIndex:     0,
		Data:        []byte{01, 02, 03, 04},
	}
	raw_log_2, err := event.NewSpaghettiLog(suite.network_id, suite.timestamp, &eth_log_2)
	suite.Require().NoError(err)

	err = SetLogs(&suite.block, []eth_types.Log{eth_log, eth_log_1, eth_log_2})
	suite.Require().NoError(err)
	suite.Require().EqualValues(suite.block.RawLogs[0], *raw_log)
	suite.Require().EqualValues(suite.block.RawLogs[1], *raw_log_1)
	suite.Require().EqualValues(suite.block.RawLogs[2], *raw_log_2)
}

func (suite *TestBlockSuite) TestGetSmartcontract() {
	one_log := "0xC6EF8A96F20d50E347eD9a1C84142D02b1EFedc0"
	two_logs := "0xFEEF8A96F20d50E347eD9a1C84142D02b1EFedc0"
	no_logs := "0xFFEF8A96F20d50E347eD9a1C84142D02b1EFedc0"

	logs := suite.block.GetForSmartcontract(one_log)
	suite.Require().Len(logs, 1)

	logs = suite.block.GetForSmartcontract(two_logs)
	suite.Require().Len(logs, 2)

	logs = suite.block.GetForSmartcontract(no_logs)
	suite.Require().Len(logs, 0)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestBlock(t *testing.T) {
	suite.Run(t, new(TestBlockSuite))
}
