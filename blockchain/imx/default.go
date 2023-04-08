// Package imx enables the support of ImmutableX (https://x.immutable.com/) by SDS.
//
// In order to enable the ImmutableX (a.k.a 'imx') based blockchains
// define the network in SDS_BLOCKCHAIN_NETWORKS configuration.
//
// The defined network should have the "type" property with "imx" value.
//
// For the network with "imx" type,
// SDS Spaghetti service will run a new service using this package.
//
// For more information about the SDS_BLOCKCHAIN_NETWORKS configuration:
// [pkg/github.com/blocklords/sds/blockchain/network.SDS_BLOCKCHAIN_NETWORKS]
package imx

import (
	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/common/data_type/key_value"
)

// Default values for imx specific configurations
var ImxConfiguration = configuration.DefaultConfig{
	Title: "ImmutableX Network",
	Parameters: key_value.New(map[string]interface{}{
		REQUEST_PER_SECOND: uint64(20),
	}),
}
