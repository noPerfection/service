package smartcontract

import (
	"github.com/blocklords/sds/static/smartcontract/key"
)

type Smartcontract struct {
	NetworkId        string `json:"network_id"`
	Address          string `json:"address"`
	AbiHash          string `json:"abi_hash"`
	TransactionId    string `json:"transaction_id"`
	TransactionIndex uint   `json:"transaction_index"`
	Deployer         string `json:"deployer"`
	BlockNumber      uint64 `json:"pre_deploy_block_number"`
	BlockTimestamp   uint64 `json:"pre_deploy_block_timestamp"`
}

// Get the unique smartcontract key within the SeascapeSDS.
//
// For more information about smartonctract keys check:
//
// “gosds/static/smartcontract/key“
func (c *Smartcontract) Key() key.Key {
	return key.New(c.NetworkId, c.Address)
}
