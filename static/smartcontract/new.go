package smartcontract

import (
	"github.com/blocklords/gosds/common/data_type/key_value"
)

// Creates a new smartcontract from the JSON
func New(parameters key_value.KeyValue) (*Smartcontract, error) {
	var sm Smartcontract
	err := parameters.ToInterface(&sm)
	if err != nil {
		return nil, err
	}

	return &sm, nil
}
