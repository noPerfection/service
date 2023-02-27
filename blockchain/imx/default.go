package imx

import (
	"github.com/blocklords/gosds/app/configuration"
	"github.com/blocklords/gosds/common/data_type/key_value"
)

var ImxConfiguration = configuration.DefaultConfig{
	Title: "ImmutableX Network",
	Parameters: key_value.New(map[string]interface{}{
		REQUEST_PER_SECOND: uint64(20),
	}),
}
