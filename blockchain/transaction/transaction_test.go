package transaction

import (
	"fmt"
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
type TestTransactionSuite struct {
	suite.Suite
	tx RawTransaction
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
func (suite *TestTransactionSuite) SetupTest() {
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
	tx := RawTransaction{
		SmartcontractKey: sm_key,
		BlockHeader:      header,
		TransactionKey:   tx_key,
		From:             from,
		Data:             data,
		Value:            value,
	}

	suite.tx = tx
}

func (suite *TestTransactionSuite) TestToString() {
	expected := `{"block_header":{"block_number":1,"block_timestamp":2},"smartcontract_key":{"address":"0xdead","network_id":"1"},"transaction_data":"asdsad","transaction_from":"0x123","transaction_key":{"id":"0x123213","index":0}}`
	actual, err := suite.tx.ToString()
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
	tx := RawTransaction{
		SmartcontractKey: smartcontract_key.Key{},
		BlockHeader:      header,
		TransactionKey:   tx_key,
		From:             from,
		Data:             data,
		Value:            value,
	}
	// one of the parameters is empty
	_, err = tx.ToString()
	suite.Require().Error(err)

	// omitting the parameter
	tx = RawTransaction{
		BlockHeader:    header,
		TransactionKey: tx_key,
		From:           from,
		Data:           data,
		Value:          value,
	}
	_, err = tx.ToString()
	suite.Require().Error(err)

	// from is empty, it should fail
	sm_key := smartcontract_key.Key{
		NetworkId: "1",
		Address:   "0xdead",
	}
	tx = RawTransaction{
		SmartcontractKey: sm_key,
		BlockHeader:      header,
		TransactionKey:   tx_key,
		From:             "",
		Data:             data,
		Value:            value,
	}
	_, err = tx.ToString()
	suite.Require().Error(err)

	// one of the nested keys is empty
	// in this case tx.SmartcontractKey.NetworkId
	tx = RawTransaction{
		SmartcontractKey: smartcontract_key.Key{
			Address: "0xdead",
		},
		BlockHeader:    header,
		TransactionKey: tx_key,
		From:           "asdsa",
		Data:           data,
		Value:          value,
	}
	_, err = tx.ToString()
	suite.Require().Error(err)
}

func (suite *TestTransactionSuite) TestNew() {
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
	kv := key_value.Empty().
		Set("smartcontract_key", sm_key).
		Set("block_header", header).
		Set("transaction_key", tx_key).
		Set("transaction_from", from)

	_, err := New(kv)
	suite.Require().NoError(err)

	// empty map key should fail
	// as transaction.validate() will fail
	kv = key_value.Empty()
	_, err = New(kv)
	suite.Require().Error(err)

	// one of the parameters is missing
	// here its missing to have "transaction_from"
	kv = key_value.Empty().
		Set("smartcontract_key", sm_key).
		Set("block_header", header).
		Set("transaction_key", tx_key).
		Set("from", from)
	_, err = New(kv)
	suite.Require().Error(err)

	// even if the nested parameter is invalid
	// its an error
	kv = key_value.Empty().
		Set("smartcontract_key", "it should be map string").
		Set("block_header", header).
		Set("transaction_key", tx_key).
		Set("from", from)
	_, err = New(kv)
	suite.Require().Error(err)

	// empty value should fail
	kv = key_value.Empty().
		Set("smartcontract_key", map[string]interface{}{}).
		Set("block_header", header).
		Set("transaction_key", tx_key).
		Set("from", from)
	_, err = New(kv)
	suite.Require().Error(err)

	// using a key value instead the data type should be valid
	// its valid
	kv = key_value.Empty().
		Set("smartcontract_key", key_value.Empty().
			Set("address", "0xdead").
			Set("network_id", "1"),
		).
		Set("block_header", header).
		Set("transaction_key", tx_key).
		Set("transaction_from", from)
	fmt.Println("the kv to convert", kv)
	_, err = New(kv)
	suite.Require().NoError(err)

	// using a map instead data type for the field
	// should be valid
	kv = key_value.Empty().
		Set("smartcontract_key", map[string]interface{}{
			"address":    "0xdead",
			"network_id": "1",
		}).
		Set("block_header", header).
		Set("transaction_key", tx_key).
		Set("transaction_from", from)
	_, err = New(kv)
	suite.Require().NoError(err)

	// one of the nested parameters is empty
	// it should fail
	kv = key_value.Empty().
		Set("smartcontract_key", key_value.Empty().
			Set("address", "0xdead")).
		Set("block_header", header).
		Set("transaction_key", tx_key).
		Set("transaction_from", from)
	_, err = New(kv)
	suite.Require().Error(err)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestTransaction(t *testing.T) {
	suite.Run(t, new(TestTransactionSuite))
}
