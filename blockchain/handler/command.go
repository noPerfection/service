package handler

import (
	"github.com/blocklords/sds/app/communication/command"
)

const (
	// The command executed by the blockchain/network manager
	FILTER_LOG_COMMAND command.CommandName = "log-filter"
	// The command executed by the blockchain/network manager
	TRANSACTION_COMMAND command.CommandName = "transaction"
	// The command executed by the blockchain manager
	DEPLOYED_TRANSACTION_COMMAND command.CommandName = "transaction_deployed_get"
	// The command executed by the blockchain manager
	NETWORK_IDS_COMMAND command.CommandName = "network_id_get_all"
	// The command executed by the blockchain manager
	NETWORKS_COMMAND command.CommandName = "network_get_all"
	// The command executed by the blockchain manager
	NETWORK_COMMAND command.CommandName = "network_get"
	// Internal blockchain package's.
	// Its only for EVM blockchains.
	//
	// This command used by two packages.
	//
	//  1. evm/indexer/recent uses it to fetch the most block number
	//     from blockchain client.
	//  2. evm/indexer/recent uses it to push new recent block number
	//     to old indexer and indexer manager.
	RECENT_BLOCK_NUMBER command.CommandName = "recent-block-number"
	// Internal from SDS Indexer service to SDS Blockchain service
	NEW_CATEGORIZED_SMARTCONTRACTS command.CommandName = "new-smartcontracts"
)
