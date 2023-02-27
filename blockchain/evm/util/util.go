package util

import (
	"fmt"
	"math/big"
	"strings"

	eth_parameters "github.com/ethereum/go-ethereum/params"
)

// https://github.com/ethereum/go-ethereum/issues/21221
func WeiToEther(wei *big.Int) *big.Float {
	return new(big.Float).Quo(new(big.Float).SetInt(wei), big.NewFloat(eth_parameters.Ether))
}

func EtherToWei(eth *big.Float) *big.Int {
	truncInt, _ := eth.Int(nil)
	truncInt = new(big.Int).Mul(truncInt, big.NewInt(eth_parameters.Ether))
	fracStr := strings.Split(fmt.Sprintf("%.18f", eth), ".")[1]
	fracStr += strings.Repeat("0", 18-len(fracStr))
	fracInt, _ := new(big.Int).SetString(fracStr, 10)
	wei := new(big.Int).Add(truncInt, fracInt)
	return wei
}

// parse_big_float parse string value to big.Float
func ParseBigFloat(value string) (*big.Float, error) {
	f := new(big.Float)
	f.SetPrec(236) //  IEEE 754 octuple-precision binary floating-point format: binary256
	f.SetMode(big.ToNearestEven)
	_, err := fmt.Sscan(value, f)
	return f, err
}
