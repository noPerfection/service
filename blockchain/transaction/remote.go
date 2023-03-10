package transaction

import (
	"fmt"

	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/common/blockchain"
)

// Sends the command to the remote SDS Spaghetti to get the smartcontract deploy metaData by
// its transaction id
// todo return transaction index
func RemoteTransactionDeployed(socket *remote.Socket, network_id string, Txid string) (string, string, blockchain.BlockHeader, error) {
	// Send hello.
	request := message.Request{
		Command: "transaction_deployed_get",
		Parameters: map[string]interface{}{
			"network_id": network_id,
			"txid":       Txid,
		},
	}

	raw_params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return "", "", blockchain.BlockHeader{}, fmt.Errorf("socket.RequestRemoteService: %w", err)
	}

	var transaction RawTransaction
	err = raw_params.ToInterface(&transaction)
	if err != nil {
		return "", "", blockchain.BlockHeader{}, fmt.Errorf("key-value to interface: %w", err)
	}

	return transaction.SmartcontractKey.Address, transaction.From, transaction.BlockHeader, nil
}
