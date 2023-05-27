package abi

import (
	"testing"

	"github.com/blocklords/sds/common/data_type"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/stretchr/testify/suite"
)

// We won't test the requests.
// The requests are tested in the controllers
// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestAbiSuite struct {
	suite.Suite
	abi Abi
}

func (suite *TestAbiSuite) SetupTest() {
	bytes := []byte(`[{
			"type": "constructor",
			"inputs": [],
			"stateMutability": "nonpayable"
		},
		{
			"name": "Approval",
			"type": "event",
			"inputs": [
				{
					"name": "owner",
					"type": "address",
					"indexed": true,
					"internalType": "address"
				},
				{
					"name": "approved",
					"type": "address",
					"indexed": true,
					"internalType": "address"
				},
				{
					"name": "tokenId",
					"type": "uint256",
					"indexed": true,
					"internalType": "uint256"
				}
			],
			"anonymous": false
		},
		{
			"name": "ApprovalForAll",
			"type": "event",
			"inputs": [
				{
					"name": "owner",
					"type": "address",
					"indexed": true,
					"internalType": "address"
				},
				{
					"name": "operator",
					"type": "address",
					"indexed": true,
					"internalType": "address"
				},
				{
					"name": "approved",
					"type": "bool",
					"indexed": false,
					"internalType": "bool"
				}
			],
			"anonymous": false
		},
		{
			"name": "Minted",
			"type": "event",
			"inputs": [
				{
					"name": "owner",
					"type": "address",
					"indexed": true,
					"internalType": "address"
				},
				{
					"name": "id",
					"type": "uint256",
					"indexed": true,
					"internalType": "uint256"
				},
				{
					"name": "generation",
					"type": "uint256",
					"indexed": false,
					"internalType": "uint256"
				},
				{
					"name": "quality",
					"type": "uint8",
					"indexed": false,
					"internalType": "uint8"
				}
			],
			"anonymous": false
		},
		{
			"name": "OwnershipTransferred",
			"type": "event",
			"inputs": [
				{
					"name": "previousOwner",
					"type": "address",
					"indexed": true,
					"internalType": "address"
				},
				{
					"name": "newOwner",
					"type": "address",
					"indexed": true,
					"internalType": "address"
				}
			],
			"anonymous": false
		},
		{
			"name": "Transfer",
			"type": "event",
			"inputs": [
				{
					"name": "from",
					"type": "address",
					"indexed": true,
					"internalType": "address"
				},
				{
					"name": "to",
					"type": "address",
					"indexed": true,
					"internalType": "address"
				},
				{
					"name": "tokenId",
					"type": "uint256",
					"indexed": true,
					"internalType": "uint256"
				}
			],
			"anonymous": false
		},
		{
			"name": "approve",
			"type": "function",
			"inputs": [
				{
					"name": "to",
					"type": "address",
					"internalType": "address"
				},
				{
					"name": "tokenId",
					"type": "uint256",
					"internalType": "uint256"
				}
			],
			"outputs": [],
			"stateMutability": "nonpayable"
		},
		{
			"name": "balanceOf",
			"type": "function",
			"inputs": [
				{
					"name": "owner",
					"type": "address",
					"internalType": "address"
				}
			],
			"outputs": [
				{
					"name": "",
					"type": "uint256",
					"internalType": "uint256"
				}
			],
			"stateMutability": "view"
		},
		{
			"name": "baseURI",
			"type": "function",
			"inputs": [],
			"outputs": [
				{
					"name": "",
					"type": "string",
					"internalType": "string"
				}
			],
			"stateMutability": "view"
		},
		{
			"name": "burn",
			"type": "function",
			"inputs": [
				{
					"name": "tokenId",
					"type": "uint256",
					"internalType": "uint256"
				}
			],
			"outputs": [],
			"stateMutability": "nonpayable"
		},
		{
			"name": "getApproved",
			"type": "function",
			"inputs": [
				{
					"name": "tokenId",
					"type": "uint256",
					"internalType": "uint256"
				}
			],
			"outputs": [
				{
					"name": "",
					"type": "address",
					"internalType": "address"
				}
			],
			"stateMutability": "view"
		},
		{
			"name": "isApprovedForAll",
			"type": "function",
			"inputs": [
				{
					"name": "owner",
					"type": "address",
					"internalType": "address"
				},
				{
					"name": "operator",
					"type": "address",
					"internalType": "address"
				}
			],
			"outputs": [
				{
					"name": "",
					"type": "bool",
					"internalType": "bool"
				}
			],
			"stateMutability": "view"
		},
		{
			"name": "name",
			"type": "function",
			"inputs": [],
			"outputs": [
				{
					"name": "",
					"type": "string",
					"internalType": "string"
				}
			],
			"stateMutability": "view"
		},
		{
			"name": "owner",
			"type": "function",
			"inputs": [],
			"outputs": [
				{
					"name": "",
					"type": "address",
					"internalType": "address"
				}
			],
			"stateMutability": "view"
		},
		{
			"name": "ownerOf",
			"type": "function",
			"inputs": [
				{
					"name": "tokenId",
					"type": "uint256",
					"internalType": "uint256"
				}
			],
			"outputs": [
				{
					"name": "",
					"type": "address",
					"internalType": "address"
				}
			],
			"stateMutability": "view"
		},
		{
			"name": "paramsOf",
			"type": "function",
			"inputs": [
				{
					"name": "",
					"type": "uint256",
					"internalType": "uint256"
				}
			],
			"outputs": [
				{
					"name": "quality",
					"type": "uint256",
					"internalType": "uint256"
				},
				{
					"name": "generation",
					"type": "uint8",
					"internalType": "uint8"
				}
			],
			"stateMutability": "view"
		},
		{
			"name": "renounceOwnership",
			"type": "function",
			"inputs": [],
			"outputs": [],
			"stateMutability": "nonpayable"
		},
		{
			"name": "safeTransferFrom",
			"type": "function",
			"inputs": [
				{
					"name": "from",
					"type": "address",
					"internalType": "address"
				},
				{
					"name": "to",
					"type": "address",
					"internalType": "address"
				},
				{
					"name": "tokenId",
					"type": "uint256",
					"internalType": "uint256"
				}
			],
			"outputs": [],
			"stateMutability": "nonpayable"
		},
		{
			"name": "safeTransferFrom",
			"type": "function",
			"inputs": [
				{
					"name": "from",
					"type": "address",
					"internalType": "address"
				},
				{
					"name": "to",
					"type": "address",
					"internalType": "address"
				},
				{
					"name": "tokenId",
					"type": "uint256",
					"internalType": "uint256"
				},
				{
					"name": "_data",
					"type": "bytes",
					"internalType": "bytes"
				}
			],
			"outputs": [],
			"stateMutability": "nonpayable"
		},
		{
			"name": "setApprovalForAll",
			"type": "function",
			"inputs": [
				{
					"name": "operator",
					"type": "address",
					"internalType": "address"
				},
				{
					"name": "approved",
					"type": "bool",
					"internalType": "bool"
				}
			],
			"outputs": [],
			"stateMutability": "nonpayable"
		},
		{
			"name": "supportsInterface",
			"type": "function",
			"inputs": [
				{
					"name": "interfaceId",
					"type": "bytes4",
					"internalType": "bytes4"
				}
			],
			"outputs": [
				{
					"name": "",
					"type": "bool",
					"internalType": "bool"
				}
			],
			"stateMutability": "view"
		},
		{
			"name": "symbol",
			"type": "function",
			"inputs": [],
			"outputs": [
				{
					"name": "",
					"type": "string",
					"internalType": "string"
				}
			],
			"stateMutability": "view"
		},
		{
			"name": "tokenByIndex",
			"type": "function",
			"inputs": [
				{
					"name": "index",
					"type": "uint256",
					"internalType": "uint256"
				}
			],
			"outputs": [
				{
					"name": "",
					"type": "uint256",
					"internalType": "uint256"
				}
			],
			"stateMutability": "view"
		},
		{
			"name": "tokenOfOwnerByIndex",
			"type": "function",
			"inputs": [
				{
					"name": "owner",
					"type": "address",
					"internalType": "address"
				},
				{
					"name": "index",
					"type": "uint256",
					"internalType": "uint256"
				}
			],
			"outputs": [
				{
					"name": "",
					"type": "uint256",
					"internalType": "uint256"
				}
			],
			"stateMutability": "view"
		},
		{
			"name": "tokenURI",
			"type": "function",
			"inputs": [
				{
					"name": "tokenId",
					"type": "uint256",
					"internalType": "uint256"
				}
			],
			"outputs": [
				{
					"name": "",
					"type": "string",
					"internalType": "string"
				}
			],
			"stateMutability": "view"
		},
		{
			"name": "totalSupply",
			"type": "function",
			"inputs": [],
			"outputs": [
				{
					"name": "",
					"type": "uint256",
					"internalType": "uint256"
				}
			],
			"stateMutability": "view"
		},
		{
			"name": "transferFrom",
			"type": "function",
			"inputs": [
				{
					"name": "from",
					"type": "address",
					"internalType": "address"
				},
				{
					"name": "to",
					"type": "address",
					"internalType": "address"
				},
				{
					"name": "tokenId",
					"type": "uint256",
					"internalType": "uint256"
				}
			],
			"outputs": [],
			"stateMutability": "nonpayable"
		},
		{
			"name": "transferOwnership",
			"type": "function",
			"inputs": [
				{
					"name": "newOwner",
					"type": "address",
					"internalType": "address"
				}
			],
			"outputs": [],
			"stateMutability": "nonpayable"
		},
		{
			"name": "mint",
			"type": "function",
			"inputs": [
				{
					"name": "_to",
					"type": "address",
					"internalType": "address"
				},
				{
					"name": "_generation",
					"type": "uint256",
					"internalType": "uint256"
				},
				{
					"name": "_quality",
					"type": "uint8",
					"internalType": "uint8"
				}
			],
			"outputs": [
				{
					"name": "",
					"type": "uint256",
					"internalType": "uint256"
				}
			],
			"stateMutability": "nonpayable"
		},
		{
			"name": "setOwner",
			"type": "function",
			"inputs": [
				{
					"name": "_owner",
					"type": "address",
					"internalType": "address"
				}
			],
			"outputs": [],
			"stateMutability": "nonpayable"
		},
		{
			"name": "setFactory",
			"type": "function",
			"inputs": [
				{
					"name": "_factory",
					"type": "address",
					"internalType": "address"
				}
			],
			"outputs": [],
			"stateMutability": "nonpayable"
		},
		{
			"name": "setBaseUri",
			"type": "function",
			"inputs": [
				{
					"name": "_uri",
					"type": "string",
					"internalType": "string"
				}
			],
			"outputs": [],
			"stateMutability": "nonpayable"
		}
	]`)
	abi := Abi{
		Bytes: bytes,
	}
	suite.abi = abi
}

func (suite *TestAbiSuite) TestId() {
	id := `972a8818360c3c28`
	err := suite.abi.GenerateId()
	suite.Require().NoError(err)
	suite.Equal(suite.abi.Id, id)

	// Both formatted or unformatted parts are equal
	bytes := []byte(`[{"type":"constructor","inputs":[],"stateMutability":"nonpayable"},{"name":"Approval","type":"event","inputs":[{"name":"owner","type":"address","indexed":true,"internalType":"address"},{"name":"approved","type":"address","indexed":true,"internalType":"address"},{"name":"tokenId","type":"uint256","indexed":true,"internalType":"uint256"}],"anonymous":false},{"name":"ApprovalForAll","type":"event","inputs":[{"name":"owner","type":"address","indexed":true,"internalType":"address"},{"name":"operator","type":"address","indexed":true,"internalType":"address"},{"name":"approved","type":"bool","indexed":false,"internalType":"bool"}],"anonymous":false},{"name":"Minted","type":"event","inputs":[{"name":"owner","type":"address","indexed":true,"internalType":"address"},{"name":"id","type":"uint256","indexed":true,"internalType":"uint256"},{"name":"generation","type":"uint256","indexed":false,"internalType":"uint256"},{"name":"quality","type":"uint8","indexed":false,"internalType":"uint8"}],"anonymous":false},{"name":"OwnershipTransferred","type":"event","inputs":[{"name":"previousOwner","type":"address","indexed":true,"internalType":"address"},{"name":"newOwner","type":"address","indexed":true,"internalType":"address"}],"anonymous":false},{"name":"Transfer","type":"event","inputs":[{"name":"from","type":"address","indexed":true,"internalType":"address"},{"name":"to","type":"address","indexed":true,"internalType":"address"},{"name":"tokenId","type":"uint256","indexed":true,"internalType":"uint256"}],"anonymous":false},{"name":"approve","type":"function","inputs":[{"name":"to","type":"address","internalType":"address"},{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"balanceOf","type":"function","inputs":[{"name":"owner","type":"address","internalType":"address"}],"outputs":[{"name":"","type":"uint256","internalType":"uint256"}],"stateMutability":"view"},{"name":"baseURI","type":"function","inputs":[],"outputs":[{"name":"","type":"string","internalType":"string"}],"stateMutability":"view"},{"name":"burn","type":"function","inputs":[{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"getApproved","type":"function","inputs":[{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"","type":"address","internalType":"address"}],"stateMutability":"view"},{"name":"isApprovedForAll","type":"function","inputs":[{"name":"owner","type":"address","internalType":"address"},{"name":"operator","type":"address","internalType":"address"}],"outputs":[{"name":"","type":"bool","internalType":"bool"}],"stateMutability":"view"},{"name":"name","type":"function","inputs":[],"outputs":[{"name":"","type":"string","internalType":"string"}],"stateMutability":"view"},{"name":"owner","type":"function","inputs":[],"outputs":[{"name":"","type":"address","internalType":"address"}],"stateMutability":"view"},{"name":"ownerOf","type":"function","inputs":[{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"","type":"address","internalType":"address"}],"stateMutability":"view"},{"name":"paramsOf","type":"function","inputs":[{"name":"","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"quality","type":"uint256","internalType":"uint256"},{"name":"generation","type":"uint8","internalType":"uint8"}],"stateMutability":"view"},{"name":"renounceOwnership","type":"function","inputs":[],"outputs":[],"stateMutability":"nonpayable"},{"name":"safeTransferFrom","type":"function","inputs":[{"name":"from","type":"address","internalType":"address"},{"name":"to","type":"address","internalType":"address"},{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"safeTransferFrom","type":"function","inputs":[{"name":"from","type":"address","internalType":"address"},{"name":"to","type":"address","internalType":"address"},{"name":"tokenId","type":"uint256","internalType":"uint256"},{"name":"_data","type":"bytes","internalType":"bytes"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"setApprovalForAll","type":"function","inputs":[{"name":"operator","type":"address","internalType":"address"},{"name":"approved","type":"bool","internalType":"bool"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"supportsInterface","type":"function","inputs":[{"name":"interfaceId","type":"bytes4","internalType":"bytes4"}],"outputs":[{"name":"","type":"bool","internalType":"bool"}],"stateMutability":"view"},{"name":"symbol","type":"function","inputs":[],"outputs":[{"name":"","type":"string","internalType":"string"}],"stateMutability":"view"},{"name":"tokenByIndex","type":"function","inputs":[{"name":"index","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"","type":"uint256","internalType":"uint256"}],"stateMutability":"view"},{"name":"tokenOfOwnerByIndex","type":"function","inputs":[{"name":"owner","type":"address","internalType":"address"},{"name":"index","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"","type":"uint256","internalType":"uint256"}],"stateMutability":"view"},{"name":"tokenURI","type":"function","inputs":[{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"","type":"string","internalType":"string"}],"stateMutability":"view"},{"name":"totalSupply","type":"function","inputs":[],"outputs":[{"name":"","type":"uint256","internalType":"uint256"}],"stateMutability":"view"},{"name":"transferFrom","type":"function","inputs":[{"name":"from","type":"address","internalType":"address"},{"name":"to","type":"address","internalType":"address"},{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"transferOwnership","type":"function","inputs":[{"name":"newOwner","type":"address","internalType":"address"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"mint","type":"function","inputs":[{"name":"_to","type":"address","internalType":"address"},{"name":"_generation","type":"uint256","internalType":"uint256"},{"name":"_quality","type":"uint8","internalType":"uint8"}],"outputs":[{"name":"","type":"uint256","internalType":"uint256"}],"stateMutability":"nonpayable"},{"name":"setOwner","type":"function","inputs":[{"name":"_owner","type":"address","internalType":"address"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"setFactory","type":"function","inputs":[{"name":"_factory","type":"address","internalType":"address"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"setBaseUri","type":"function","inputs":[{"name":"_uri","type":"string","internalType":"string"}],"outputs":[],"stateMutability":"nonpayable"}]`)
	abi := Abi{
		Bytes: bytes,
	}
	abi.GenerateId()
	suite.Equal(abi.Id, id)
}

func (suite *TestAbiSuite) TestToString() {
	// Both formatted or unformatted parts are equal
	bytes := []byte(`[{"type":"constructor","inputs":[],"stateMutability":"nonpayable"},{"name":"Approval","type":"event","inputs":[{"name":"owner","type":"address","indexed":true,"internalType":"address"},{"name":"approved","type":"address","indexed":true,"internalType":"address"},{"name":"tokenId","type":"uint256","indexed":true,"internalType":"uint256"}],"anonymous":false},{"name":"ApprovalForAll","type":"event","inputs":[{"name":"owner","type":"address","indexed":true,"internalType":"address"},{"name":"operator","type":"address","indexed":true,"internalType":"address"},{"name":"approved","type":"bool","indexed":false,"internalType":"bool"}],"anonymous":false},{"name":"Minted","type":"event","inputs":[{"name":"owner","type":"address","indexed":true,"internalType":"address"},{"name":"id","type":"uint256","indexed":true,"internalType":"uint256"},{"name":"generation","type":"uint256","indexed":false,"internalType":"uint256"},{"name":"quality","type":"uint8","indexed":false,"internalType":"uint8"}],"anonymous":false},{"name":"OwnershipTransferred","type":"event","inputs":[{"name":"previousOwner","type":"address","indexed":true,"internalType":"address"},{"name":"newOwner","type":"address","indexed":true,"internalType":"address"}],"anonymous":false},{"name":"Transfer","type":"event","inputs":[{"name":"from","type":"address","indexed":true,"internalType":"address"},{"name":"to","type":"address","indexed":true,"internalType":"address"},{"name":"tokenId","type":"uint256","indexed":true,"internalType":"uint256"}],"anonymous":false},{"name":"approve","type":"function","inputs":[{"name":"to","type":"address","internalType":"address"},{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"balanceOf","type":"function","inputs":[{"name":"owner","type":"address","internalType":"address"}],"outputs":[{"name":"","type":"uint256","internalType":"uint256"}],"stateMutability":"view"},{"name":"baseURI","type":"function","inputs":[],"outputs":[{"name":"","type":"string","internalType":"string"}],"stateMutability":"view"},{"name":"burn","type":"function","inputs":[{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"getApproved","type":"function","inputs":[{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"","type":"address","internalType":"address"}],"stateMutability":"view"},{"name":"isApprovedForAll","type":"function","inputs":[{"name":"owner","type":"address","internalType":"address"},{"name":"operator","type":"address","internalType":"address"}],"outputs":[{"name":"","type":"bool","internalType":"bool"}],"stateMutability":"view"},{"name":"name","type":"function","inputs":[],"outputs":[{"name":"","type":"string","internalType":"string"}],"stateMutability":"view"},{"name":"owner","type":"function","inputs":[],"outputs":[{"name":"","type":"address","internalType":"address"}],"stateMutability":"view"},{"name":"ownerOf","type":"function","inputs":[{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"","type":"address","internalType":"address"}],"stateMutability":"view"},{"name":"paramsOf","type":"function","inputs":[{"name":"","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"quality","type":"uint256","internalType":"uint256"},{"name":"generation","type":"uint8","internalType":"uint8"}],"stateMutability":"view"},{"name":"renounceOwnership","type":"function","inputs":[],"outputs":[],"stateMutability":"nonpayable"},{"name":"safeTransferFrom","type":"function","inputs":[{"name":"from","type":"address","internalType":"address"},{"name":"to","type":"address","internalType":"address"},{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"safeTransferFrom","type":"function","inputs":[{"name":"from","type":"address","internalType":"address"},{"name":"to","type":"address","internalType":"address"},{"name":"tokenId","type":"uint256","internalType":"uint256"},{"name":"_data","type":"bytes","internalType":"bytes"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"setApprovalForAll","type":"function","inputs":[{"name":"operator","type":"address","internalType":"address"},{"name":"approved","type":"bool","internalType":"bool"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"supportsInterface","type":"function","inputs":[{"name":"interfaceId","type":"bytes4","internalType":"bytes4"}],"outputs":[{"name":"","type":"bool","internalType":"bool"}],"stateMutability":"view"},{"name":"symbol","type":"function","inputs":[],"outputs":[{"name":"","type":"string","internalType":"string"}],"stateMutability":"view"},{"name":"tokenByIndex","type":"function","inputs":[{"name":"index","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"","type":"uint256","internalType":"uint256"}],"stateMutability":"view"},{"name":"tokenOfOwnerByIndex","type":"function","inputs":[{"name":"owner","type":"address","internalType":"address"},{"name":"index","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"","type":"uint256","internalType":"uint256"}],"stateMutability":"view"},{"name":"tokenURI","type":"function","inputs":[{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"","type":"string","internalType":"string"}],"stateMutability":"view"},{"name":"totalSupply","type":"function","inputs":[],"outputs":[{"name":"","type":"uint256","internalType":"uint256"}],"stateMutability":"view"},{"name":"transferFrom","type":"function","inputs":[{"name":"from","type":"address","internalType":"address"},{"name":"to","type":"address","internalType":"address"},{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"transferOwnership","type":"function","inputs":[{"name":"newOwner","type":"address","internalType":"address"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"mint","type":"function","inputs":[{"name":"_to","type":"address","internalType":"address"},{"name":"_generation","type":"uint256","internalType":"uint256"},{"name":"_quality","type":"uint8","internalType":"uint8"}],"outputs":[{"name":"","type":"uint256","internalType":"uint256"}],"stateMutability":"nonpayable"},{"name":"setOwner","type":"function","inputs":[{"name":"_owner","type":"address","internalType":"address"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"setFactory","type":"function","inputs":[{"name":"_factory","type":"address","internalType":"address"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"setBaseUri","type":"function","inputs":[{"name":"_uri","type":"string","internalType":"string"}],"outputs":[],"stateMutability":"nonpayable"}]`)
	abi := Abi{
		Bytes: bytes,
	}
	suite.Equal(suite.abi.ToString(), abi.ToString())

	bytes = []byte(`its not a valid json`)
	abi = Abi{Bytes: bytes}
	suite.Empty(abi.ToString())
}

func (suite *TestAbiSuite) TestNew() {
	bytes := []byte(`[{"type":"constructor","inputs":[],"stateMutability":"nonpayable"},{"name":"Approval","type":"event","inputs":[{"name":"owner","type":"address","indexed":true,"internalType":"address"},{"name":"approved","type":"address","indexed":true,"internalType":"address"},{"name":"tokenId","type":"uint256","indexed":true,"internalType":"uint256"}],"anonymous":false},{"name":"ApprovalForAll","type":"event","inputs":[{"name":"owner","type":"address","indexed":true,"internalType":"address"},{"name":"operator","type":"address","indexed":true,"internalType":"address"},{"name":"approved","type":"bool","indexed":false,"internalType":"bool"}],"anonymous":false},{"name":"Minted","type":"event","inputs":[{"name":"owner","type":"address","indexed":true,"internalType":"address"},{"name":"id","type":"uint256","indexed":true,"internalType":"uint256"},{"name":"generation","type":"uint256","indexed":false,"internalType":"uint256"},{"name":"quality","type":"uint8","indexed":false,"internalType":"uint8"}],"anonymous":false},{"name":"OwnershipTransferred","type":"event","inputs":[{"name":"previousOwner","type":"address","indexed":true,"internalType":"address"},{"name":"newOwner","type":"address","indexed":true,"internalType":"address"}],"anonymous":false},{"name":"Transfer","type":"event","inputs":[{"name":"from","type":"address","indexed":true,"internalType":"address"},{"name":"to","type":"address","indexed":true,"internalType":"address"},{"name":"tokenId","type":"uint256","indexed":true,"internalType":"uint256"}],"anonymous":false},{"name":"approve","type":"function","inputs":[{"name":"to","type":"address","internalType":"address"},{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"balanceOf","type":"function","inputs":[{"name":"owner","type":"address","internalType":"address"}],"outputs":[{"name":"","type":"uint256","internalType":"uint256"}],"stateMutability":"view"},{"name":"baseURI","type":"function","inputs":[],"outputs":[{"name":"","type":"string","internalType":"string"}],"stateMutability":"view"},{"name":"burn","type":"function","inputs":[{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"getApproved","type":"function","inputs":[{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"","type":"address","internalType":"address"}],"stateMutability":"view"},{"name":"isApprovedForAll","type":"function","inputs":[{"name":"owner","type":"address","internalType":"address"},{"name":"operator","type":"address","internalType":"address"}],"outputs":[{"name":"","type":"bool","internalType":"bool"}],"stateMutability":"view"},{"name":"name","type":"function","inputs":[],"outputs":[{"name":"","type":"string","internalType":"string"}],"stateMutability":"view"},{"name":"owner","type":"function","inputs":[],"outputs":[{"name":"","type":"address","internalType":"address"}],"stateMutability":"view"},{"name":"ownerOf","type":"function","inputs":[{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"","type":"address","internalType":"address"}],"stateMutability":"view"},{"name":"paramsOf","type":"function","inputs":[{"name":"","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"quality","type":"uint256","internalType":"uint256"},{"name":"generation","type":"uint8","internalType":"uint8"}],"stateMutability":"view"},{"name":"renounceOwnership","type":"function","inputs":[],"outputs":[],"stateMutability":"nonpayable"},{"name":"safeTransferFrom","type":"function","inputs":[{"name":"from","type":"address","internalType":"address"},{"name":"to","type":"address","internalType":"address"},{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"safeTransferFrom","type":"function","inputs":[{"name":"from","type":"address","internalType":"address"},{"name":"to","type":"address","internalType":"address"},{"name":"tokenId","type":"uint256","internalType":"uint256"},{"name":"_data","type":"bytes","internalType":"bytes"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"setApprovalForAll","type":"function","inputs":[{"name":"operator","type":"address","internalType":"address"},{"name":"approved","type":"bool","internalType":"bool"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"supportsInterface","type":"function","inputs":[{"name":"interfaceId","type":"bytes4","internalType":"bytes4"}],"outputs":[{"name":"","type":"bool","internalType":"bool"}],"stateMutability":"view"},{"name":"symbol","type":"function","inputs":[],"outputs":[{"name":"","type":"string","internalType":"string"}],"stateMutability":"view"},{"name":"tokenByIndex","type":"function","inputs":[{"name":"index","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"","type":"uint256","internalType":"uint256"}],"stateMutability":"view"},{"name":"tokenOfOwnerByIndex","type":"function","inputs":[{"name":"owner","type":"address","internalType":"address"},{"name":"index","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"","type":"uint256","internalType":"uint256"}],"stateMutability":"view"},{"name":"tokenURI","type":"function","inputs":[{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[{"name":"","type":"string","internalType":"string"}],"stateMutability":"view"},{"name":"totalSupply","type":"function","inputs":[],"outputs":[{"name":"","type":"uint256","internalType":"uint256"}],"stateMutability":"view"},{"name":"transferFrom","type":"function","inputs":[{"name":"from","type":"address","internalType":"address"},{"name":"to","type":"address","internalType":"address"},{"name":"tokenId","type":"uint256","internalType":"uint256"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"transferOwnership","type":"function","inputs":[{"name":"newOwner","type":"address","internalType":"address"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"mint","type":"function","inputs":[{"name":"_to","type":"address","internalType":"address"},{"name":"_generation","type":"uint256","internalType":"uint256"},{"name":"_quality","type":"uint8","internalType":"uint8"}],"outputs":[{"name":"","type":"uint256","internalType":"uint256"}],"stateMutability":"nonpayable"},{"name":"setOwner","type":"function","inputs":[{"name":"_owner","type":"address","internalType":"address"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"setFactory","type":"function","inputs":[{"name":"_factory","type":"address","internalType":"address"}],"outputs":[],"stateMutability":"nonpayable"},{"name":"setBaseUri","type":"function","inputs":[{"name":"_uri","type":"string","internalType":"string"}],"outputs":[],"stateMutability":"nonpayable"}]`)
	abi := Abi{
		Bytes: bytes,
	}
	err := abi.GenerateId()
	suite.Require().NoError(err)

	new_abi, err := NewFromBytes(bytes)
	suite.Require().NoError(err)
	suite.Require().EqualValues(abi, *new_abi)

	var json interface{}
	err = abi.Interface(&json)
	suite.Require().NoError(err)

	// passing the interface without reference
	// should return an error
	var no_ref interface{}
	err = abi.Interface(no_ref)
	suite.Require().Error(err)

	// we can't pass the pointer to the interface
	_, err = NewFromInterface(&json)
	suite.Require().Error(err)

	json_abi, err := NewFromInterface(json)
	suite.Require().NoError(err)
	suite.Require().EqualValues(abi, *json_abi)

	kv := key_value.Empty().
		Set("id", abi.Id).
		Set("bytes", data_type.AddJsonPrefix(abi.Bytes))
	kv_abi, err := New(kv)
	suite.Require().NoError(err)
	suite.Require().EqualValues(abi, *kv_abi)

	// passing the value with a missing parameter should fail
	// in this case its an bytes.
	kv = key_value.Empty().
		Set("id", abi.Id)
	_, err = New(kv)
	suite.Require().Error(err)

	kv = key_value.Empty().
		Set("id", "").
		Set("bytes", abi.Bytes)
	_, err = New(kv)
	suite.Require().Error(err)

	kv = key_value.Empty().
		Set("id", abi.Id).
		Set("bytes", []byte(``))
	_, err = New(kv)
	suite.Require().Error(err)

	kv = key_value.Empty().
		Set("id", abi.Id).
		Set("bytes", []byte(`not a valid json`))
	_, err = New(kv)
	suite.Require().Error(err)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestAbi(t *testing.T) {
	suite.Run(t, new(TestAbiSuite))
}
