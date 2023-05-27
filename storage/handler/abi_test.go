package handler

import (
	"testing"

	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/service/communication/message"
	"github.com/blocklords/sds/service/log"
	"github.com/blocklords/sds/storage/abi"
	"github.com/stretchr/testify/suite"
)

// We won't test the requests.
// The requests are tested in the controllers
// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestAbiSuite struct {
	suite.Suite
	logger   log.Logger
	abi_0_id string
	abi_1_id string
	abi      abi.Abi
	abi_list *key_value.List
}

func (suite *TestAbiSuite) SetupTest() {
	logger, err := log.New("test", log.WITH_TIMESTAMP)
	suite.Require().NoError(err)
	suite.logger = logger

	suite.abi_0_id = "hello"
	suite.abi_1_id = "abi_id_1"

	bytes := []byte(`[{"type":"constructor","inputs":[],"stateMutability":"nonpayable"},{"name":"Approval","type":"event","inputs":[{"name":"owner","type":"address","indexed":true,"internalType":"address"},{"name":"approved","type":"address","indexed":true,"internalType":"address"},{"name":"tokenId","type":"uint256","indexed":true,"internalType":"uint256"}],"anonymous":false},{"name":"ApprovalForAll","type":"event","inputs":[{"name":"owner","type":"address","indexed":true,"internalType":"address"},{"name":"operator","type":"address","indexed":true,"internalType":"address"},{"name":"approved","type":"bool","indexed":false,"internalType":"bool"}],"anonymous":false},{"name":"Minted","type":"event","inputs":[{"name":"owner","type":"address","indexed":true,"internalType":"address"},{"name":"id","type":"uint256","indexed":true,"internalType":"uint256"},{"name":"generation","type":"uint256","indexed":false,"internalType":"uint256"},{"name":"quality","type":"uint8","indexed":false,"internalType":"uint8"}],"anonymous":false},{"name":"OwnershipTransferred","type":"event","inputs":[{"name":"previousOwner","type":"address","indexed":true,"internalType":"address"},{"name":"newOwner","type":"address","indexed":true,"internalType":"address"}],"anonymous":false},{"name":"Transfer","type":"event","inputs":[{"name":"from","type":"address","indexed":true,"internalType":"address"},{"name":"to","type":"address","indexed":true,"internalType":"address"},{"name":"tokenId","type":"uint256","indexed":true,"internalType":"uint256"}],"anonymous":false},{"name":"approve","type":"function","inputs":[{"name":"to","type":"address","internalType":"address"},{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"balanceOf","type":"function","inputs":[{"name":"owner","type":"address","internalType":"address"}],"outputs":[{"name":"","type":"uint256","internalType":"uint256"}],"stateMutability":"view"},{"name":"baseURI","type":"function","inputs":[],"outputs":[{"name":"","type":"string","internalType":"string"}],"stateMutability":"view"},{"name":"burn","type":"function","inputs":[{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"getApproved","type":"function","inputs":[{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"","type":"address","internalType":"address"}],"stateMutability":"view"},{"name":"isApprovedForAll","type":"function","inputs":[{"name":"owner","type":"address","internalType":"address"},{"name":"operator","type":"address","internalType":"address"}],"outputs":[{"name":"","type":"bool","internalType":"bool"}],"stateMutability":"view"},{"name":"name","type":"function","inputs":[],"outputs":[{"name":"","type":"string","internalType":"string"}],"stateMutability":"view"},{"name":"owner","type":"function","inputs":[],"outputs":[{"name":"","type":"address","internalType":"address"}],"stateMutability":"view"},{"name":"ownerOf","type":"function","inputs":[{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"","type":"address","internalType":"address"}],"stateMutability":"view"},{"name":"paramsOf","type":"function","inputs":[{"name":"","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"quality","type":"uint256","internalType":"uint256"},{"name":"generation","type":"uint8","internalType":"uint8"}],"stateMutability":"view"},{"name":"renounceOwnership","type":"function","inputs":[],"outputs":[],"stateMutability":"nonpayable"},{"name":"safeTransferFrom","type":"function","inputs":[{"name":"from","type":"address","internalType":"address"},{"name":"to","type":"address","internalType":"address"},{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"safeTransferFrom","type":"function","inputs":[{"name":"from","type":"address","internalType":"address"},{"name":"to","type":"address","internalType":"address"},{"name":"tokenId","type":"uint256","internalType":"uint256"},{"name":"_data","type":"bytes","internalType":"bytes"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"setApprovalForAll","type":"function","inputs":[{"name":"operator","type":"address","internalType":"address"},{"name":"approved","type":"bool","internalType":"bool"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"supportsInterface","type":"function","inputs":[{"name":"interfaceId","type":"bytes4","internalType":"bytes4"}],"outputs":[{"name":"","type":"bool","internalType":"bool"}],"stateMutability":"view"},{"name":"symbol","type":"function","inputs":[],"outputs":[{"name":"","type":"string","internalType":"string"}],"stateMutability":"view"},{"name":"tokenByIndex","type":"function","inputs":[{"name":"index","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"","type":"uint256","internalType":"uint256"}],"stateMutability":"view"},{"name":"tokenOfOwnerByIndex","type":"function","inputs":[{"name":"owner","type":"address","internalType":"address"},{"name":"index","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"","type":"uint256","internalType":"uint256"}],"stateMutability":"view"},{"name":"tokenURI","type":"function","inputs":[{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"","type":"string","internalType":"string"}],"stateMutability":"view"},{"name":"totalSupply","type":"function","inputs":[],"outputs":[{"name":"","type":"uint256","internalType":"uint256"}],"stateMutability":"view"},{"name":"transferFrom","type":"function","inputs":[{"name":"from","type":"address","internalType":"address"},{"name":"to","type":"address","internalType":"address"},{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"transferOwnership","type":"function","inputs":[{"name":"newOwner","type":"address","internalType":"address"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"mint","type":"function","inputs":[{"name":"_to","type":"address","internalType":"address"},{"name":"_generation","type":"uint256","internalType":"uint256"},{"name":"_quality","type":"uint8","internalType":"uint8"}],"outputs":[{"name":"","type":"uint256","internalType":"uint256"}],"stateMutability":"nonpayable"},{"name":"setOwner","type":"function","inputs":[{"name":"_owner","type":"address","internalType":"address"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"setFactory","type":"function","inputs":[{"name":"_factory","type":"address","internalType":"address"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"setBaseUri","type":"function","inputs":[{"name":"_uri","type":"string","internalType":"string"}],"outputs":[],"stateMutability":"nonpayable"}]`)
	abi_0 := abi.Abi{
		Bytes: bytes,
		Id:    "hello",
	}
	suite.abi = abi_0

	abi_1 := abi.Abi{
		Bytes: []byte(`[{}]`),
		Id:    suite.abi_1_id,
	}

	list := key_value.NewList()
	err = list.Add(suite.abi_0_id, &abi_0)
	suite.Require().NoError(err)

	err = list.Add(suite.abi_1_id, &abi_1)
	suite.Require().NoError(err)
	suite.abi_list = list
}

func (suite *TestAbiSuite) TestGet() {
	// valid request
	valid_request := GetAbiRequest{
		Id: suite.abi_0_id,
	}
	valid_kv, err := key_value.NewFromInterface(valid_request)
	suite.Require().NoError(err)

	request := message.Request{
		Command:    "",
		Parameters: valid_kv,
	}
	reply := AbiGet(request, suite.logger, nil, suite.abi_list)
	suite.Require().True(reply.IsOK())

	var replied_abi GetAbiReply
	err = reply.Parameters.ToInterface(&replied_abi)
	suite.Require().NoError(err)

	suite.Require().EqualValues(suite.abi, replied_abi)

	// request with empty parameter should fail
	request = message.Request{
		Command:    "",
		Parameters: key_value.Empty(),
	}
	reply = AbiGet(request, suite.logger, nil, suite.abi_list)
	suite.Require().False(reply.IsOK())

	// request of abi that doesn't exist in the list
	// should fail
	request = message.Request{
		Command: "",
		Parameters: key_value.Empty().
			Set("abi_id", "item"),
	}
	reply = AbiGet(request, suite.logger, nil, suite.abi_list)
	suite.Require().False(reply.IsOK())

	// requesting with invalid type for abi id should fail
	request = message.Request{
		Command: "",
		Parameters: key_value.Empty().
			Set("abi_id", 123),
	}
	reply = AbiGet(request, suite.logger, nil, suite.abi_list)
	suite.Require().False(reply.IsOK())
}

func (suite *TestAbiSuite) TestSet() {
	// valid request
	valid_request := SetAbiRequest{
		Body: []string{},
	}
	valid_kv, err := key_value.NewFromInterface(valid_request)
	suite.Require().NoError(err)

	request := message.Request{
		Command:    "",
		Parameters: valid_kv,
	}
	reply := AbiRegister(request, suite.logger, nil, suite.abi_list)
	suite.Require().True(reply.IsOK())

	var replied_abi SetAbiReply
	err = reply.Parameters.ToInterface(&replied_abi)
	suite.Require().NoError(err)
	suite.Require().EqualValues(`[]`, string(replied_abi.Bytes))
	var replied_body []string
	err = replied_abi.Interface(&replied_body)
	suite.Require().NoError(err)
	suite.Require().EqualValues([]string{}, replied_body)

	// the abi list should have the item
	abi_in_list, err := suite.abi_list.Get(replied_abi.Id)
	suite.Require().NoError(err)
	suite.Require().EqualValues(&replied_abi, abi_in_list)

	// registering with empty parameter should fail
	request = message.Request{
		Command:    "",
		Parameters: key_value.Empty(),
	}
	reply = AbiRegister(request, suite.logger, nil, suite.abi_list)
	suite.Require().False(reply.IsOK())

	// request of abi that already exist in the list
	// should fail
	request = message.Request{
		Command:    "",
		Parameters: valid_kv,
	}
	reply = AbiRegister(request, suite.logger, nil, suite.abi_list)
	suite.Require().False(reply.IsOK())
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestAbi(t *testing.T) {
	suite.Run(t, new(TestAbiSuite))
}
