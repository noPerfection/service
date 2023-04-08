// Package evm enables the support of Ethereum and EVM based
// blockchains by SDS.
//
// In order to enable the evm based blockchains
// define the network in SDS_BLOCKCHAIN_NETWORKS configuration.
//
// The defined network should have the "type" property with "evm" value.
//
// For the network with "evm" type,
// SDS Spaghetti service will run a new service using this package.
//
// For more information about the SDS_BLOCKCHAIN_NETWORKS configuration:
// [pkg/github.com/blocklords/sds/blockchain/network.SDS_BLOCKCHAIN_NETWORKS]
package evm
