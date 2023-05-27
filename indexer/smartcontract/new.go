package smartcontract

import (
	"fmt"

	"github.com/blocklords/sds/common/data_type/key_value"
)

func New(blob key_value.KeyValue) (*Smartcontract, error) {
	var sm Smartcontract
	err := blob.ToInterface(&sm)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize indexer.Smartcontract key-value %v to intermediate interface: %v", blob, err)
	}

	if err := sm.Validate(); err != nil {
		return nil, fmt.Errorf("sm.Validate(): %w", err)
	}

	return &sm, nil
}
