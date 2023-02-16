package transaction

import (
	"github.com/blocklords/gosds/message"
	"github.com/blocklords/gosds/remote"
)

// Sends the command to the remote SDS Spaghetti to get the smartcontract deploy metaData by
// its transaction id
func RemoteTransactionDeployed(socket *remote.Socket, network_id string, Txid string) (string, string, uint64, uint64, error) {
	// Send hello.
	request := message.Request{
		Command: "transaction_deployed_get",
		Parameters: map[string]interface{}{
			"network_id": network_id,
			"txid":       Txid,
		},
	}

	params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return "", "", 0, 0, err
	}

	address, err := message.GetString(params, "address")
	if err != nil {
		return "", "", 0, 0, err
	}
	deployer, err := message.GetString(params, "deployer")
	if err != nil {
		return "", "", 0, 0, err
	}
	block_number, err := message.GetUint64(params, "block_number")
	if err != nil {
		return "", "", 0, 0, err
	}
	block_timestamp, err := message.GetUint64(params, "block_timestamp")
	if err != nil {
		return "", "", 0, 0, err
	}

	return address, deployer, block_number, block_timestamp, nil
}
