package abi

import (
	"testing"

	"github.com/blocklords/sds/static/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/suite"
)

// We won't test the requests.
// The requests are tested in the controllers
// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestAbiSuite struct {
	suite.Suite
	abi      *Abi
	event_id string
}

// used for testing
// https://bscscan.com/tx/0x9565744fc676d421681ecc588446e0d0ae5627bf618a5688f7772adcf6667c81#eventlog
func (suite *TestAbiSuite) SetupTest() {
	bytes := []byte(`[{
			"name": "ApprovalForAll","type": "event","inputs": [{
				"name": "owner","type": "address","indexed": true,"internalType": "address"},{
				"name": "operator","type": "address","indexed": true,"internalType": "address"},{
				"name": "approved","type": "bool","indexed": false,"internalType": "bool"}
			],"anonymous": false}]`)
	static_abi := abi.Abi{
		Bytes: bytes,
	}
	err := static_abi.GenerateId()
	suite.Require().NoError(err)
	abi, err := NewFromStatic(&static_abi)
	suite.Require().NoError(err)
	suite.abi = abi

	suite.event_id = "0x17307eab39ab6107e8899845ad3d59bd9653f200f220920489ca2b5937696c31"
}

func (suite *TestAbiSuite) TestInternals() {
	// without prefix it should fail
	event_id := "17307eab39ab6107e8899845ad3d59bd9653f200f220920489ca2b5937696c31"
	no_events := suite.abi.get_events(event_id)
	suite.Require().Len(no_events, 0)

	// event id is different
	// here we changed first byte from 1 to 2
	event_id = "0x27307eab39ab6107e8899845ad3d59bd9653f200f220920489ca2b5937696c31"
	no_events = suite.abi.get_events(event_id)
	suite.Require().Len(no_events, 0)

	// invalid event id
	event_id = "not an event"
	no_events = suite.abi.get_events(event_id)
	suite.Require().Len(no_events, 0)

	// valid event id
	events := suite.abi.get_events(suite.event_id)
	suite.Require().Len(events, 1)

	// non indexed parameters
	args := get_indexed(events[0].Inputs.NonIndexed())
	suite.Require().Len(args, 0)

	// nil means no event inputs
	args = get_indexed(nil)
	suite.Require().Len(args, 0)

	// we have two indexed parameters
	args = get_indexed(events[0].Inputs)
	suite.Require().Len(args, 2)
}

func (suite *TestAbiSuite) TestDecoding() {
	topics := []string{
		suite.event_id,
		"0x000000000000000000000000b7e957790ea36c7eac30464de74f13770fd6da8a",
		"0x00000000000000000000000029b0d9a9a989e4651488d0002ebf79199ce1b7c1",
	}
	data := "0000000000000000000000000000000000000000000000000000000000000001"
	event_name, args, err := suite.abi.DecodeLog(topics, data)
	suite.Require().NoError(err)
	suite.Require().EqualValues("ApprovalForAll", event_name)
	suite.Require().Len(args, 3)

	expected_owner := "0xb7E957790Ea36C7EAC30464dE74F13770fd6dA8A"
	expected_operator := "0x29b0d9A9A989e4651488D0002ebf79199cE1b7C1"
	expected_approved := true

	owner, ok := args["owner"].(common.Address)
	suite.Require().True(ok)
	suite.Require().EqualValues(expected_owner, owner.Hex())

	operator, ok := args["operator"].(common.Address)
	suite.Require().True(ok)
	suite.Require().EqualValues(expected_operator, operator.Hex())

	approved, ok := args["approved"].(bool)
	suite.Require().True(ok)
	suite.Require().EqualValues(expected_approved, approved)
}

func (suite *TestAbiSuite) TestNew() {
	// empty abi should fail
	bytes := []byte(`[]`)
	static_abi := abi.Abi{
		Bytes: bytes,
	}
	_, err := NewFromStatic(&static_abi)
	suite.Require().NoError(err)

	// empty string instead json should fail
	bytes = []byte(``)
	static_abi = abi.Abi{
		Bytes: bytes,
	}
	_, err = NewFromStatic(&static_abi)
	suite.Require().Error(err)

	// invalid abi type
	bytes = []byte(`{"a":{"b":1}}`)
	static_abi = abi.Abi{
		Bytes: bytes,
	}
	_, err = NewFromStatic(&static_abi)
	suite.Require().Error(err)

	// invalid json
	bytes = []byte(`[{},{},]`)
	static_abi = abi.Abi{
		Bytes: bytes,
	}
	_, err = NewFromStatic(&static_abi)
	suite.Require().Error(err)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestAbi(t *testing.T) {
	suite.Run(t, new(TestAbiSuite))
}
