package configuration

import (
	"fmt"

	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/topic"
)

// Converts the Topic to the Configuration
// Note that you should set the address as well
func NewFromTopic(t topic.Topic, address string) (*Configuration, error) {
	c := &Configuration{
		Topic:   t,
		Address: address,
	}
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("Validate: %w", err)
	}

	return c, nil
}

// Creates a new storage.Configuration class based on the given data
func New(parameters key_value.KeyValue) (*Configuration, error) {
	var conf Configuration
	err := parameters.ToInterface(&conf)
	if err != nil {
		return nil, fmt.Errorf("failed to convert key-value of Configuration to interface %v", err)
	}

	if err := conf.Validate(); err != nil {
		return nil, fmt.Errorf("Validate: %w", err)
	}

	return &conf, nil
}
