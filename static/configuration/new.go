package configuration

import (
	"fmt"

	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/topic"
)

// Converts the Topic to the Configuration
func NewFromTopic(t topic.Topic) (*Configuration, error) {
	return &Configuration{
		Topic: t,
	}, nil
}

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

	return &conf, nil
}
