// Package util defines the additional functions specific for imx network
package util

import (
	"fmt"
	"math/big"
	"strings"

	imx_api "github.com/immutable/imx-core-sdk-golang/imx/api"
)

// Erc20Amount converts the transfer value parameter from big.Int to float64
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
