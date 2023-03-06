package imx

import (
	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/common/data_type/key_value"
)

var ImxConfiguration = configuration.DefaultConfig{
	Title: "ImmutableX Network",
	Parameters: key_value.New(map[string]interface{}{
		REQUEST_PER_SECOND: uint64(20),
	}),
}
