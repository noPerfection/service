package configuration

import (
	"github.com/ahmetson/common-lib/data_type/key_value"
)

// DefaultConfig Default configuration for the package
type DefaultConfig struct {
	Title      string             // package title
	Parameters key_value.KeyValue // parameters
}
