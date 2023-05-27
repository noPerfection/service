package handler

import (
	"testing"

	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/smartcontract_key"
	"github.com/blocklords/sds/service/communication/message"
	"github.com/blocklords/sds/service/log"
	"github.com/blocklords/sds/storage/smartcontract"
	"github.com/stretchr/testify/suite"
)

// We won't test the requests.
// The requests are tested in the controllers
// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestSmartcontractSuite struct {
	suite.Suite
	logger   log.Logger
	abi_0_id string
	sm_0_key smartcontract_key.Key
	sm_1_key smartcontract_key.Key
	sm       smartcontract.Smartcontract
	sm_list  *key_value.List
}

func (suite *TestSmartcontractSuite) SetupTest() {
	logger, err := log.New("test", log.WITH_TIMESTAMP)
	suite.Require().NoError(err)
	suite.logger = logger

	suite.abi_0_id = "hello"
	suite.sm_0_key = smartcontract_key.Key{
		NetworkId: "1",
		Address:   "0xaddress",
	}
	suite.sm_1_key = smartcontract_key.Key{
		NetworkId: "1",
		Address:   "0xsm_key",
	}

	sm_0 := smartcontract.Smartcontract{
		SmartcontractKey: suite.sm_0_key,
		AbiId:            suite.abi_0_id,
		TransactionKey: blockchain.TransactionKey{
			Id:    "0x1",
			Index: 0,
		},
		BlockHeader: blockchain.BlockHeader{
			Number:    blockchain.Number(1),
			Timestamp: blockchain.Timestamp(2),
		},
		Deployer: "0xdeployer",
	}
	suite.sm = sm_0

	sm_1 := smartcontract.Smartcontract{
		SmartcontractKey: suite.sm_1_key,
		AbiId:            suite.abi_0_id,
		TransactionKey: blockchain.TransactionKey{
			Id:    "0x1",
			Index: 0,
		},
		BlockHeader: blockchain.BlockHeader{
			Number:    blockchain.Number(1),
			Timestamp: blockchain.Timestamp(2),
		},
		Deployer: "0xdeployer",
	}

	list := key_value.NewList()
	err = list.Add(sm_0.SmartcontractKey, &sm_0)
	suite.Require().NoError(err)

	err = list.Add(sm_1.SmartcontractKey, &sm_1)
	suite.Require().NoError(err)
	suite.sm_list = list
}

func (suite *TestSmartcontractSuite) TestGet() {
	// valid request
	valid_kv, err := key_value.NewFromInterface(suite.sm_0_key)
	suite.Require().NoError(err)

	request := message.Request{
		Command:    "",
		Parameters: valid_kv,
	}
	reply := SmartcontractGet(request, suite.logger, nil, nil, suite.sm_list)
	suite.Require().True(reply.IsOK())

	var replied_sm GetSmartcontractReply
	err = reply.Parameters.ToInterface(&replied_sm)
	suite.Require().NoError(err)

	suite.Require().EqualValues(suite.sm, replied_sm)

	// request with empty parameter should fail
	request = message.Request{
		Command:    "",
		Parameters: key_value.Empty(),
	}
	reply = SmartcontractGet(request, suite.logger, nil, nil, suite.sm_list)
	suite.Require().False(reply.IsOK())

	// request of smartcontract that doesn't exist in the list
	// should fail
	request = message.Request{
		Command: "",
		Parameters: key_value.Empty().
			Set("network_id", "56").
			Set("address", "0xsm_key"),
	}
	reply = SmartcontractGet(request, suite.logger, nil, nil, suite.sm_list)
	suite.Require().False(reply.IsOK())

	// requesting with invalid type for abi id should fail
	request = message.Request{
		Command: "",
		Parameters: key_value.Empty().
			Set("network_id", 1).
			Set("address", "0xsm_key"),
	}
	reply = SmartcontractGet(request, suite.logger, nil, nil, suite.sm_list)
	suite.Require().False(reply.IsOK())
}

func (suite *TestSmartcontractSuite) TestSet() {
	// valid request
	valid_request := smartcontract.Smartcontract{
		SmartcontractKey: smartcontract_key.Key{
			NetworkId: "imx",
			Address:   "0xnft",
		},
		AbiId: suite.abi_0_id,
		TransactionKey: blockchain.TransactionKey{
			Id:    "0x1",
			Index: 0,
		},
		BlockHeader: blockchain.BlockHeader{
			Number:    blockchain.Number(1),
			Timestamp: blockchain.Timestamp(2),
		},
		Deployer: "0xdeployer",
	}
	valid_kv, err := key_value.NewFromInterface(valid_request)
	suite.Require().NoError(err)

	request := message.Request{
		Command:    "",
		Parameters: valid_kv,
	}
	reply := SmartcontractRegister(request, suite.logger, nil, nil, suite.sm_list)
	suite.T().Log(reply.Message)
	suite.Require().True(reply.IsOK())

	var replied_sm SetSmartcontractReply
	err = reply.Parameters.ToInterface(&replied_sm)
	suite.Require().NoError(err)
	suite.Require().EqualValues(valid_request, replied_sm)

	// the abi list should have the item
	sm_in_list, err := suite.sm_list.Get(replied_sm.SmartcontractKey)
	suite.Require().NoError(err)
	suite.Require().EqualValues(&replied_sm, sm_in_list)

	// registering with empty parameter should fail
	request = message.Request{
		Command:    "",
		Parameters: key_value.Empty(),
	}
	reply = SmartcontractRegister(request, suite.logger, nil, nil, suite.sm_list)
	suite.Require().False(reply.IsOK())

	// request of abi that already exist in the list
	// should fail
	request = message.Request{
		Command:    "",
		Parameters: valid_kv,
	}
	reply = SmartcontractRegister(request, suite.logger, nil, nil, suite.sm_list)
	suite.Require().False(reply.IsOK())
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestSmartcontract(t *testing.T) {
	suite.Run(t, new(TestSmartcontractSuite))
}
