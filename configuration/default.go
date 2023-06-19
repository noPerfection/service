package configuration

import (
	"github.com/Seascape-Foundation/sds-service-lib/common/data_type/key_value"
)

// Default configuration for the package
type DefaultConfig struct {
	Title      string             // package title
	Parameters key_value.KeyValue // parameters
}
