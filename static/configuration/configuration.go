package configuration

import (
	"fmt"

	"github.com/blocklords/sds/common/topic"
)

// The Configuration sets the relationship between the organization and the smartcontract.
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
