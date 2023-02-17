package smartcontract

import (
	"fmt"

	"github.com/blocklords/gosds/common/data_type/key_value"
)

func New(blob key_value.KeyValue) (*Smartcontract, error) {
	i, err := blob.ToInterface()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize categorizer.Smartcontract key-value %v to intermediate interface: %v", blob, err)
	}

	sm, ok := i.(Smartcontract)
	if !ok {
		return nil, fmt.Errorf("failed to serialize %v categorizer.Smartcontract intermediate interface to Smartcontract", blob)
	}

	return &sm, nil
}
