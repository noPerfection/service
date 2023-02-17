package configuration

import (
	"github.com/blocklords/gosds/common/data_type/key_value"
)

// Default configurations for the given package
type DefaultConfig struct {
	Title      string             // package title
	Parameters key_value.KeyValue // parameters
}
