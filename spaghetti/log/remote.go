package log

import (
	"errors"

	"github.com/blocklords/gosds/app/remote"
	"github.com/blocklords/gosds/app/remote/message"
)

// Sends the command to the remote SDS Spaghetti to filter the logs
func RemoteLogFilter(socket *remote.Socket, block_number_from uint64, addresses []string) ([]*Log, error) {
	// Send hello.
	request := message.Request{
		Command: "log-filter",
		Parameters: map[string]interface{}{
			"block_number_from": block_number_from,
			"addresses":         addresses,
		},
	}

	params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return nil, err
	}

	raw_logs, ok := params.ToMap()["logs"].([]interface{})
	if !ok {
		return nil, errors.New("no logs received from SDS Spaghetti")
	}
	logs, err := NewLogs(raw_logs)
	if err != nil {
		return nil, errors.New("failed to parse log when filtering it")
	}

	return logs, nil
}
