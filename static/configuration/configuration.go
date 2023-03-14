package configuration

import "github.com/blocklords/sds/common/topic"

// The Configuration sets the relationship between the organization and the smartcontract.
type Configuration struct {
	Topic   topic.Topic `json:"topic"`
	Address string      `json:"address"`
}
