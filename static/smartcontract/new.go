package smartcontract

import (
	"fmt"

	"github.com/blocklords/gosds/common/data_type/key_value"
)

// Creates a new smartcontract from the JSON
func New(parameters key_value.KeyValue) (*Smartcontract, error) {
	i, err := parameters.ToInterface()
	if err != nil {
		return nil, err
	}

	sm, ok := i.(Smartcontract)
	if !ok {
		return nil, fmt.Errorf("failed to convert key-value to static.Smartcontract. Intermediary interface: %v", i)
	}

	return &sm, nil
}
