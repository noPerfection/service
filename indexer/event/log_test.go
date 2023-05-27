package event

import (
	"testing"

	blockchain_event "github.com/blocklords/sds/blockchain/event"
	"github.com/blocklords/sds/blockchain/transaction"
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/smartcontract_key"
	"github.com/blocklords/sds/indexer/smartcontract"
	"github.com/stretchr/testify/suite"
)

// We won't test the requests.
// The requests are tested in the controllers
// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestLogSuite struct {
	suite.Suite
	key             smartcontract_key.Key
	transaction_key blockchain.TransactionKey
	block_header    blockchain.BlockHeader
	index           uint
	name            string
	parameters      key_value.KeyValue
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
	suite.key = smartcontract_key.Key{
		NetworkId: "1",
		Address:   "0xdead",
	}
	suite.block_header, _ = blockchain.NewHeader(uint64(1), uint64(2))
	suite.transaction_key = blockchain.TransactionKey{
		Id:    "0x123213",
		Index: 0,
	}
	suite.name = "Transfer"
	suite.parameters = key_value.Empty().
		Set("from", "rich").
		Set("to", "poor").
		Set("value", uint64(2))
}

func (suite *TestLogSuite) TestNew() {
	from := "0x123"
	data := "asdsad"
	value := float64(0.0)
	tx := transaction.RawTransaction{
		SmartcontractKey: suite.key,
		BlockHeader:      suite.block_header,
		TransactionKey:   suite.transaction_key,
		From:             from,
		Data:             data,
		Value:            value,
	}

	raw_log := blockchain_event.RawLog{
		Transaction: tx,
		Index:       0,
		Data:        "123213",
		Topics: []string{
			"indexed_signature",
			"indexed_parameter",
		},
	}

	sm := smartcontract.Smartcontract{
		SmartcontractKey: suite.key,
	}

	log := New(suite.name, suite.parameters)
	suite.Require().Error(log.Validate())
	log.AddMetadata(&raw_log)
	suite.Require().Error(log.Validate())
	log.AddSmartcontractData(&sm)
	suite.Require().NoError(log.Validate())

	// getting from the key value
	kv := key_value.Empty().
		Set("smartcontract_key", key_value.Empty().
			Set("network_id", "1").
			Set("address", "address")).
		Set("transaction_key", key_value.Empty().
			Set("transaction_id", "asdsad").
			Set("transaction_index", 0)).
		Set("block_header", key_value.Empty().
			Set("block_number", 23).
			Set("block_timestamp", 123)).
		Set("log_index", 2).
		Set("event_name", "Transfer").
		Set("event_parameters", "")
	_, err := NewFromMap(kv)
	suite.Require().Error(err)

	kv.Set("event_parameters", key_value.Empty())
	_, err = NewFromMap(kv)
	suite.Require().NoError(err)
}

// testing parsing and serializing topics

// test with the five logs
// 2 of them belongs to the same address
// others are individial

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestLog(t *testing.T) {
	suite.Run(t, new(TestLogSuite))
}
