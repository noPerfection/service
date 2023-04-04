package configuration

import (
	"fmt"

	"github.com/blocklords/sds/common/topic"
)

// The Static Configuration is the relationship
// between the topic and the smartcontract.
//
// The database part depends on the Static Smartcontract
type Configuration struct {
	Topic   topic.Topic `json:"topic"`
	Address string      `json:"address"`
}

func (c *Configuration) Validate() error {
	if err := c.Topic.Validate(); err != nil {
		return fmt.Errorf("Topic.Validate: %w", err)
	}
	if c.Topic.Level() != topic.SMARTCONTRACT_LEVEL {
		return fmt.Errorf("Topic.Level is not smartcontract level")
	}
	if len(c.Address) == 0 {
		return fmt.Errorf("missing Address parameter")
	}

	return nil
}
