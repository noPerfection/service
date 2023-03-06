package transaction

import (
	"fmt"

	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/common/data_type/key_value"
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

	raw_params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return "", "", 0, 0, fmt.Errorf("socket.RequestRemoteService: %w", err)
	}

	params := key_value.New(raw_params)

	address, err := params.GetString("address")
	if err != nil {
		return "", "", 0, 0, fmt.Errorf("params.GetString(`string`): %w", err)
	}
	deployer, err := params.GetString("deployer")
	if err != nil {
		return "", "", 0, 0, fmt.Errorf("params.GetString(`deployer`): %w", err)
	}
	block_number, err := params.GetUint64("block_number")
	if err != nil {
		return "", "", 0, 0, fmt.Errorf("params.GetUint64(`block_number`): %w", err)
	}
	block_timestamp, err := params.GetUint64("block_timestamp")
	if err != nil {
		return "", "", 0, 0, fmt.Errorf("params.GetUint64(`block_timestamp`): %w", err)
	}

	return address, deployer, block_number, block_timestamp, nil
}
