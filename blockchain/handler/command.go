package handler

import (
	"github.com/blocklords/sds/app/command"
)

const (
	// The command executed by the blockchain/network manager
	FILTER_LOG_COMMAND command.Command = "log-filter"
	// The command executed by the blockchain/network manager
	TRANSACTION_COMMAND command.Command = "transaction"
	// The command executed by the blockchain manager
	DEPLOYED_TRANSACTION_COMMAND command.Command = "transaction_deployed_get"
	// The command executed by the blockchain manager
	NETWORK_IDS_COMMAND command.Command = "network_id_get_all"
	// The command executed by the blockchain manager
	NETWORKS_COMMAND command.Command = "network_get_all"
	// The command executed by the blockchain manager
	NETWORK_COMMAND command.Command = "network_get"
	// Internal blockchain package's.
	// Its only for EVM blockchains.
	// EVM blockchain categorizer fetches the recent block number from
	// EVM blockchain clients
	RECENT_BLOCK_NUMBER command.Command = "recent-block-number"
	// Internal from SDS Categorizer service to SDS Blockchain service
	NEW_CATEGORIZED_SMARTCONTRACTS command.Command = "new-smartcontracts"
)
