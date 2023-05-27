// Package smartcontract defines the smartcontract data and the link to the abi
package smartcontract

import (
	"fmt"

	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/smartcontract_key"
)

// The storage smartcontract
// It keeps the read-only parameters such as
// associated ABI, deployer, address, block parameter as well as the transaction
// where it was deployed.
//
// The Database interaction depends on the sds/storage/abi
type Smartcontract struct {
	SmartcontractKey smartcontract_key.Key     `json:"key"`
	AbiId            string                    `json:"abi_id"`
	TransactionKey   blockchain.TransactionKey `json:"transaction_key"`
	BlockHeader      blockchain.BlockHeader    `json:"block_header"`
	Deployer         string                    `json:"deployer"`
}

func (sm *Smartcontract) Validate() error {
	err := sm.SmartcontractKey.Validate()
	if err != nil {
		return fmt.Errorf("SmartcontractKey.Validate: %w", err)
	}

	if len(sm.AbiId) == 0 {
		return fmt.Errorf("the AbiId is missing")
	}
	if len(sm.Deployer) == 0 {
		return fmt.Errorf("the Deployer is missing")
	}

	if err := sm.TransactionKey.Validate(); err != nil {
		return fmt.Errorf("TransactionKey.Validate: %w", err)
	}

	if err := sm.BlockHeader.Validate(); err != nil {
		return fmt.Errorf("BlockHeader.Validate: %w", err)
	}

	return nil
}
