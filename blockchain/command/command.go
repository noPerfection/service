package command

import (
	"github.com/blocklords/sds/app/remote/command"
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
)
