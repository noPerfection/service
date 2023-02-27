// Categorize ImmutableX (https://x.immutable.com/) blockchain data
package imx

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/blocklords/gosds/app/configuration"

	imx_api "github.com/immutable/imx-core-sdk-golang/imx/api"
)

type Manager struct {
	SmartcontractAmount uint
	DelayPerSecond      time.Duration
	request_per_second  uint64
}

const request_per_second = "SDS_IMX_REQUEST_PER_SECOND"
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

	if app_config.GetUint64(request_per_second) == 0 {
		return errors.New("invalid 'SDS_IMX_REQUEST_PER_SECOND' environment variable it should be a numeric number greater than 0")
	}

	return nil
}

// /////////////////////////////////////////////////////////////////////////////////

func NewManager(app_config *configuration.Config) *Manager {
	manager := &Manager{
		SmartcontractAmount: 0,
		request_per_second:  app_config.GetUint64(request_per_second),
	}

	manager.calculate_request_delay()

	return manager
}

// Based on total amount of smartcontracts, how long we delay to request to ImmutableX nodes
func (manager *Manager) AddSmartcontract() {
	manager.SmartcontractAmount++

	manager.calculate_request_delay()
}

// Based on total amount of smartcontracts, how long we delay to request to ImmutableX nodes
func (manager *Manager) calculate_request_delay() {
	per_second := float64(manager.request_per_second)
	amount := float64(manager.SmartcontractAmount)

	manager.DelayPerSecond = time.Duration(float64(time.Millisecond) * amount * 1000 / per_second)
}

////////////////////////////////////////////////////////////////

// converts the transfer value parameter from big int to float
func Erc20Amount(transaction_data *imx_api.TokenData) (float64, error) {
	value := 0.0
	quantity := new(big.Int)
	_, err := fmt.Sscan(transaction_data.Quantity, quantity)
	if err != nil {
		fmt.Println("failed to parse the quantity of the imx transfer error message ", err)
		fmt.Println("this kind of error is exceptional. it means imx changed their backend.")
		fmt.Println("fix asap")
		return value, fmt.Errorf("failed to parse the quantity of the imx transfer: %v", err)
	}
	if *(transaction_data.Decimals) != 0 {
		decimals := new(big.Int)
		_, err := fmt.Sscan("1" + strings.Repeat("0", int(*(transaction_data.Decimals))))
		if err != nil {
			return value, fmt.Errorf("failed to parse the decimals of the data: %v", err)
		}

		remaining := quantity.Div(quantity, decimals)
		value, _ = big.NewFloat(0.0).SetInt(remaining).Float64()
	} else {
		value, _ = big.NewFloat(0.0).SetInt(quantity).Float64()
	}

	return value, nil
}
