package smartcontract

import (
	"fmt"

	"github.com/blocklords/sds/common/data_type/key_value"
)

// Creates a new storage/smartcontract from the JSON
func New(parameters key_value.KeyValue) (*Smartcontract, error) {
	var sm Smartcontract
	err := parameters.ToInterface(&sm)
	if err != nil {
		return nil, err
	}

	err = sm.Validate()
	if err != nil {
		return nil, fmt.Errorf("Smartcontract.Validate: %w", err)
	}

	return &sm, nil
}
