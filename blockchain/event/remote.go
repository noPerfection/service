package event

import (
	"errors"
	"fmt"

	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/common/blockchain"
)

// Sends the command to the remote SDS Spaghetti to filter the logs
// block_from parameter is either block_number or block_timestamp
// depending on the blockchain
func RemoteLogFilter(socket *remote.Socket, block_from blockchain.Number, addresses []string) ([]*RawLog, uint64, error) {
	request := message.Request{
		Command: "log-filter",
		Parameters: map[string]interface{}{
			"block_from": block_from,
			"addresses":  addresses,
		},
	}

	params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return nil, 0, fmt.Errorf("socket.RequestRemoteService: %w", err)
	}

	raw_logs, ok := params.ToMap()["logs"].([]interface{})
	if !ok {
		return nil, 0, errors.New("no logs parameter")
	}
	logs, err := NewLogs(raw_logs)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to parse log when filtering it: %w", err)
	}

	block_to, _ := params.GetUint64("block_to")

	return logs, block_to, nil
}
