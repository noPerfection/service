package imx

import (
	"errors"

	"github.com/blocklords/sds/app/configuration"
)

// REQUEST_PER_SECOND is the configuration name that defines how many requests
// SDS can do to the remote RPC.
const REQUEST_PER_SECOND = "SDS_IMX_REQUEST_PER_SECOND"

// Network ID, imx network can have one network only.
const NETWORK_ID = "imx"

// PAGE_SIZE defines how many event logs we retreive from imx for categorization
const PAGE_SIZE = int32(50)

// /////////////////////////////////////////////////////////////////////////////////
// ValidateEnv validates the validness of the configurations.
//
//	SDS_IMX_REQUEST_PER_SECOND // environment variable is given or not
//
// If the imx network is supported.
func ValidateEnv(app_config *configuration.Config) error {
	app_config.SetDefaults(ImxConfiguration)

	if app_config.GetUint64(REQUEST_PER_SECOND) == 0 {
		return errors.New("invalid 'SDS_IMX_REQUEST_PER_SECOND' environment variable it should be a numeric number greater than 0")
	}

	return nil
}
