package config

import (
	"github.com/ahmetson/common-lib/data_type/key_value"
)

// DefaultConfig Default config for the package
type DefaultConfig struct {
	Title      string             // package title
	Parameters key_value.KeyValue // parameters
}
