package smartcontract

import (
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/smartcontract_key"
)

// The static smartcontract
// It keeps the read-only parameters such as
// associated ABI, deployer, address, block parameter as well as the transaction
// where it was deployed.
//
// The Database interaction depends on the sds/static/abi
type Smartcontract struct {
	SmartcontractKey smartcontract_key.Key     `json:"key"`
	AbiId            string                    `json:"abi_id"`
	TransactionKey   blockchain.TransactionKey `json:"transaction_key"`
	BlockHeader      blockchain.BlockHeader    `json:"block_header"`
	Deployer         string                    `json:"deployer"`
}
