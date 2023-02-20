package configuration

import (
	"fmt"

	"github.com/blocklords/gosds/common/data_type/key_value"
)

// Creates a new static.Configuration class based on the given data
func New(parameters key_value.KeyValue) (*Configuration, error) {
	var conf Configuration
	err := parameters.ToInterface(&conf)
	if err != nil {
		return nil, fmt.Errorf("failed to convert key-value of Configuration to interface %v", err)
	}

	address, err := parameters.GetString("address")
	if err == nil {
		conf.SetAddress(address)
	}
	id, err := parameters.GetUint64("id")
	if err == nil {
		conf.SetId(id)
	}

	return &conf, nil
}
