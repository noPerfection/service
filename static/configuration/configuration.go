package configuration

import "github.com/blocklords/sds/common/topic"

// The Configuration sets the relationship between the organization and the smartcontract.
type Configuration struct {
	Topic   topic.Topic `json:"topic"`
	address string
}

// The smartcontract address to which the configuration belongs too.
func (c *Configuration) SetAddress(address string) {
	c.address = address
}

func (c *Configuration) Address() string {
	return c.address
}
