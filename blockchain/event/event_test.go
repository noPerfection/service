package event

import (
	"testing"

	"github.com/blocklords/sds/blockchain/transaction"
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
type TestLogSuite struct {
	suite.Suite
	log RawLog
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
	sm_key := smartcontract_key.Key{
		NetworkId: "1",
		Address:   "0xdead",
	}
	header, _ := blockchain.NewHeader(uint64(1), uint64(2))
	tx_key := blockchain.TransactionKey{
		Id:    "0x123213",
		Index: 0,
	}
	from := "0x123"
	data := "asdsad"
	value := float64(0.0)
	tx := transaction.RawTransaction{
		SmartcontractKey: sm_key,
		BlockHeader:      header,
		TransactionKey:   tx_key,
		From:             from,
		Data:             data,
		Value:            value,
	}

	log := RawLog{
		Transaction: tx,
		Index:       0,
		Data:        "123213",
		Topics: []string{
			"indexed_signature",
			"indexed_parameter",
		},
	}

	suite.log = log
}

func (suite *TestLogSuite) TestToString() {
	expected := `{"log_data":"123213","log_index":0,"log_topics":["indexed_signature","indexed_parameter"],"transaction":{"block_header":{"block_number":1,"block_timestamp":2},"smartcontract_key":{"address":"0xdead","network_id":"1"},"transaction_data":"asdsad","transaction_from":"0x123","transaction_key":{"transaction_id":"0x123213","transaction_index":0}}}`
	actual, err := suite.log.ToString()
	suite.Require().NoError(err)
	suite.Require().EqualValues(expected, actual)

	header, _ := blockchain.NewHeader(uint64(1), uint64(2))
	tx_key := blockchain.TransactionKey{
		Id:    "0x123213",
		Index: 0,
	}
	from := "0x123"
	data := "asdsad"
	value := float64(0.0)
	tx := transaction.RawTransaction{
		SmartcontractKey: smartcontract_key.Key{},
		BlockHeader:      header,
		TransactionKey:   tx_key,
		From:             from,
		Data:             data,
		Value:            value,
	}
	// one of the parameters is empty
	// its empty in the transaction
	log := RawLog{
		Transaction: tx,
		Index:       0,
		Data:        "123213",
		Topics: []string{
			"indexed_signature",
			"indexed_parameter",
		},
	}
	_, err = log.ToString()
	suite.Require().Error(err)

	// omitting the parameter
	tx = transaction.RawTransaction{
		BlockHeader:    header,
		TransactionKey: tx_key,
		From:           from,
		Data:           data,
		Value:          value,
	}
	// the transaction parameter is omitted,
	// therefore it should fail
	log = RawLog{
		Transaction: tx,
		Index:       0,
		Data:        "123213",
		Topics: []string{
			"indexed_signature",
			"indexed_parameter",
		},
	}
	_, err = log.ToString()
	suite.Require().Error(err)

	// index is empty it should not fail
	// as we use 0 as a default
	sm_key := smartcontract_key.Key{
		NetworkId: "1",
		Address:   "0xdead",
	}
	tx = transaction.RawTransaction{
		SmartcontractKey: sm_key,
		BlockHeader:      header,
		TransactionKey:   tx_key,
		From:             from,
		Data:             data,
		Value:            value,
	}
	log = RawLog{
		Transaction: tx,
		Data:        "123213",
		Topics: []string{
			"indexed_signature",
			"indexed_parameter",
		},
	}
	_, err = log.ToString()
	suite.Require().NoError(err)

	// topics is omitted, its optional
	// so it's fine
	log = RawLog{
		Transaction: tx,
		Index:       0,
		Data:        "123213",
	}
	_, err = log.ToString()
	suite.Require().NoError(err)

	// Minimal log, it should be fine
	log = RawLog{
		Transaction: tx,
		Index:       0,
	}
	_, err = log.ToString()
	suite.Require().NoError(err)
}

func (suite *TestLogSuite) TestNew() {
	sm_key := smartcontract_key.Key{
		NetworkId: "1",
		Address:   "0xdead",
	}
	header, _ := blockchain.NewHeader(uint64(1), uint64(2))
	tx_key := blockchain.TransactionKey{
		Id:    "0x123213",
		Index: 0,
	}
	from := "0x123"
	tx_kv := key_value.Empty().
		Set("smartcontract_key", sm_key).
		Set("block_header", header).
		Set("transaction_key", tx_key).
		Set("transaction_from", from)
	kv := key_value.Empty().
		Set("transaction", tx_kv).
		Set("log_index", uint(0))
	_, err := New(kv)
	suite.Require().NoError(err)

	// empty map key should fail
	// as transaction.validate() will fail
	kv = key_value.Empty()
	_, err = New(kv)
	suite.Require().Error(err)

	// one of the parameters is missing
	// here its missing to have "index"
	kv = key_value.Empty().
		Set("transaction", tx_kv)
	_, err = New(kv)
	suite.Require().Error(err)

	// even if the nested parameter is invalid
	// its an error
	tx_kv = key_value.Empty().
		Set("smartcontract_key", "it should be map string").
		Set("block_header", header).
		Set("transaction_key", tx_key).
		Set("from", from)
	kv = key_value.Empty().
		Set("transaction", tx_kv).
		Set("index", uint(0))
	_, err = New(kv)
	suite.Require().Error(err)

	// empty value should fail
	kv = key_value.Empty().
		Set("transaction", map[string]interface{}{}).
		Set("index", uint(0))
	_, err = New(kv)
	suite.Require().Error(err)

	// using any number type for index should be correct
	// in this case we use int, not uint
	tx_kv = key_value.Empty().
		Set("smartcontract_key", sm_key).
		Set("block_header", header).
		Set("transaction_key", tx_key).
		Set("transaction_from", from)
	kv = key_value.Empty().
		Set("transaction", tx_kv).
		Set("log_index", int(0))
	_, err = New(kv)
	suite.Require().NoError(err)

	// number could not be represented as a string
	kv = key_value.Empty().
		Set("transaction", tx_kv).
		Set("log_index", "0")
	_, err = New(kv)
	suite.Require().Error(err)

	// however a incorrect format is not allowed
	kv = key_value.Empty().
		Set("transaction", tx_kv).
		Set("log_index", "must be a number")
	_, err = New(kv)
	suite.Require().Error(err)
}

// testing parsing and serializing topics
func (suite *TestLogSuite) TestTopics() {
	expected := `["indexed_signature","indexed_parameter"]`
	actual := suite.log.TopicRaw()
	suite.Require().EqualValues(expected, string(actual))

	topics := []string{
		"indexed_signature",
		"indexed_parameter",
	}

	log := RawLog{}
	err := log.ParseTopics([]byte(expected))
	suite.Require().NoError(err)
	suite.Require().EqualValues(topics, log.Topics)
}

// test with the five logs
// 2 of them belongs to the same address
// others are individial
func (suite *TestLogSuite) TestLogList() {
	all_log := make([]RawLog, 5)
	sm_key := smartcontract_key.Key{
		NetworkId: "1",
		Address:   "0x1",
	}
	header, _ := blockchain.NewHeader(uint64(1), uint64(2))
	tx_key := blockchain.TransactionKey{
		Id:    "0x123213",
		Index: 0,
	}
	from := "0x123"
	data := "asdsad"
	value := float64(0.0)
	tx := transaction.RawTransaction{
		SmartcontractKey: sm_key,
		BlockHeader:      header,
		TransactionKey:   tx_key,
		From:             from,
		Data:             data,
		Value:            value,
	}

	// the first one
	log := RawLog{
		Transaction: tx,
		Index:       0,
		Data:        "123213",
		Topics: []string{
			"indexed_signature",
			"indexed_parameter",
		},
	}
	all_log[0] = log

	// second
	sm_key = smartcontract_key.Key{
		NetworkId: "1",
		Address:   "0x1",
	}
	header, _ = blockchain.NewHeader(uint64(2), uint64(3))
	tx.BlockHeader = header
	tx.SmartcontractKey = sm_key
	log = RawLog{
		Transaction: tx,
		Index:       0,
		Data:        "123213",
		Topics: []string{
			"indexed_signature",
			"indexed_parameter",
		},
	}
	all_log[1] = log

	// third
	sm_key = smartcontract_key.Key{
		NetworkId: "1",
		Address:   "0x2",
	}
	header, _ = blockchain.NewHeader(uint64(3), uint64(4))
	tx.BlockHeader = header
	tx.SmartcontractKey = sm_key
	log = RawLog{
		Transaction: tx,
		Index:       0,
		Data:        "123213",
		Topics: []string{
			"indexed_signature",
			"indexed_parameter",
		},
	}
	all_log[2] = log

	// fourth
	sm_key = smartcontract_key.Key{
		NetworkId: "1",
		Address:   "0x3",
	}
	latest_header, _ := blockchain.NewHeader(uint64(4), uint64(5))
	tx.BlockHeader = latest_header
	tx.SmartcontractKey = sm_key
	log = RawLog{
		Transaction: tx,
		Index:       0,
		Data:        "123213",
		Topics: []string{
			"indexed_signature",
			"indexed_parameter",
		},
	}
	all_log[3] = log

	// fifth
	sm_key = smartcontract_key.Key{
		NetworkId: "1",
		Address:   "0x4",
	}
	header, _ = blockchain.NewHeader(uint64(3), uint64(4))
	tx.BlockHeader = header
	tx.SmartcontractKey = sm_key
	log = RawLog{
		Transaction: tx,
		Index:       0,
		Data:        "123213",
		Topics: []string{
			"indexed_signature",
			"indexed_parameter",
		},
	}
	all_log[4] = log

	// Testing the filtering by address
	filtered_logs := FilterByAddress(all_log, "0x1")
	suite.Require().Len(filtered_logs, 2)

	filtered_logs = FilterByAddress(all_log, "0x2")
	suite.Require().Len(filtered_logs, 1)

	filtered_logs = FilterByAddress(all_log, "0x5")
	suite.Require().Len(filtered_logs, 0)

	// Testing the most recent block
	recent_block := RecentBlock(all_log)
	suite.Require().EqualValues(latest_header, recent_block)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestLog(t *testing.T) {
	suite.Run(t, new(TestLogSuite))
}
