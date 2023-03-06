package smartcontract

import (
	"fmt"

	"github.com/blocklords/sds/common/data_type/key_value"
)

func New(blob key_value.KeyValue) (*Smartcontract, error) {
	var sm Smartcontract
	err := blob.ToInterface(&sm)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize categorizer.Smartcontract key-value %v to intermediate interface: %v", blob, err)
	}

	return &sm, nil
}
