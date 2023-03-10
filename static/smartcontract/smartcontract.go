package smartcontract

import (
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/smartcontract_key"
)

type Smartcontract struct {
	SmartcontractKey smartcontract_key.Key     `json:"key"`
	AbiId            string                    `json:"abi_id"`
	TransactionKey   blockchain.TransactionKey `json:"transaction_key"`
	BlockHeader      blockchain.BlockHeader    `json:"block_header"`
	Deployer         string                    `json:"deployer"`
}
