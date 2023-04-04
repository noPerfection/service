package smartcontract

import (
	"fmt"

	"github.com/blocklords/sds/common/data_type/key_value"
)

// Creates a new static/smartcontract from the JSON
func New(parameters key_value.KeyValue) (*Smartcontract, error) {
	var sm Smartcontract
	err := parameters.ToInterface(&sm)
	if err != nil {
		return nil, err
	}

	err = sm.SmartcontractKey.Validate()
	if err != nil {
		return nil, fmt.Errorf("SmartcontractKey.Validate: %w", err)
	}

	if len(sm.AbiId) == 0 {
		return nil, fmt.Errorf("the AbiId is missing")
	}
	if len(sm.Deployer) == 0 {
		return nil, fmt.Errorf("the Deployer is missing")
	}

	if err := sm.TransactionKey.Validate(); err != nil {
		return nil, fmt.Errorf("TransactionKey.Validate: %w", err)
	}

	if err := sm.BlockHeader.Validate(); err != nil {
		return nil, fmt.Errorf("BlockHeader.Validate: %w", err)
	}

	return &sm, nil
}
