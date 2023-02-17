package log

import (
	"github.com/blocklords/gosds/app/remote"
	"github.com/blocklords/gosds/app/remote/message"
)

// Return list of logs for the transaction keys from the remote SDS Categorizer.
// For the transaction keys see
// github.com/blocklords/gosds/categorizer/transaction.go TransactionKey()
func RemoteLogs(socket *remote.Socket, keys []string) ([]*Log, error) {
	request := message.Request{
		Command: "log_get_all",
		Parameters: map[string]interface{}{
			"keys": keys,
		},
	}
	params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return nil, err
	}

	raw_logs, err := message.GetMapList(params, "logs")
	if err != nil {
		return nil, err
	}

	logs := make([]*Log, len(raw_logs))
	for i, raw := range raw_logs {
		log, err := NewFromMap(raw)
		if err != nil {
			return nil, err
		}
		logs[i] = log
	}

	return logs, nil
}

// Parse the raw event data using SDS Log.
// parsing events using JSON abi is harder in golang, therefore we use javascript
// implementation called SDS Log.
func RemoteLogParse(socket *remote.Socket, network_id string, address string, data string, topics []string) (string, map[string]interface{}, error) {
	request := message.Request{
		Command: "parse",
		Parameters: map[string]interface{}{
			"network_id": network_id,
			"address":    address,
			"data":       data,
			"topics":     topics,
		},
	}

	params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return "", nil, err
	}

	name, err := message.GetString(params, "name")
	if err != nil {
		return "", nil, err
	}
	args, err := message.GetMap(params, "args")
	if err != nil {
		return "", nil, err
	}

	return name, args, nil
}
