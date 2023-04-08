// Package util has the additonal functions specific to EVM blockchains only.
package util

import (
	"fmt"
	"math/big"
	"strings"

	eth_parameters "github.com/ethereum/go-ethereum/params"
)

// Converts the number in WEI format to ETH format.
//
// 1 ETH = 1e18 WEI.
//
// https://eth-converter.com/
// https://github.com/ethereum/go-ethereum/issues/21221
//
// Example:
//
//		wei := big.NewInt(100000000000000000)
//		eth := WeiToEther(wei)
//		// prints 0.1
//	 	fmt.Println(eth.Float64())
func WeiToEther(wei *big.Int) *big.Float {
	eth := new(big.Float).Quo(new(big.Float).SetInt(wei), big.NewFloat(eth_parameters.Ether))
	return eth
}

// EtherToWei is opposite of WeiToEther it converts the ETH to WEI format.
func EtherToWei(eth *big.Float) *big.Int {
	truncInt, _ := eth.Int(nil)
	truncInt = new(big.Int).Mul(truncInt, big.NewInt(eth_parameters.Ether))
	fracStr := strings.Split(fmt.Sprintf("%.18f", eth), ".")[1]
	fracStr += strings.Repeat("0", 18-len(fracStr))
	fracInt, _ := new(big.Int).SetString(fracStr, 10)
	wei := new(big.Int).Add(truncInt, fracInt)
	return wei
}

// ParseBigFloat parses the string value to big.Float
//
// Useful to track the attached ETH to the transactions when they are saved in the SDS Database.
func ParseBigFloat(value string) (*big.Float, error) {
	f := new(big.Float)
	f.SetPrec(236) //  IEEE 754 octuple-precision binary floating-point format: binary256
	f.SetMode(big.ToNearestEven)
	_, err := fmt.Sscan(value, f)

	if err != nil {
		return nil, fmt.Errorf("failed to parse '%s' string to big.Float: %w", value, err)
	}

	return f, nil
}
