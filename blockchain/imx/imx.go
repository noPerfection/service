// Categorize ImmutableX (https://x.immutable.com/) blockchain data
package imx

import (
	"errors"

	"github.com/blocklords/gosds/app/configuration"
)

const REQUEST_PER_SECOND = "SDS_IMX_REQUEST_PER_SECOND"
const NETWORK_ID = "imx"
const PAGE_SIZE = int32(50)

// /////////////////////////////////////////////////////////////////////////////////
// Checks whetehr the immutable environment variables set
//
//	the request_per_second environment variable is given or not
//
// If the imx network is supported.
func ValidateEnv(app_config *configuration.Config) error {
	app_config.SetDefaults(ImxConfiguration)

	if app_config.GetUint64(REQUEST_PER_SECOND) == 0 {
		return errors.New("invalid 'SDS_IMX_REQUEST_PER_SECOND' environment variable it should be a numeric number greater than 0")
	}

	return nil
}
