package event

import (
	"fmt"

	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/smartcontract_key"
)

// Returns list of logs for the given smartcontract keys
func RemoteSnapshot(socket *remote.Socket, smartcontract_keys []smartcontract_key.Key, block_timestamp blockchain.Timestamp) ([]*Log, blockchain.Timestamp, error) {
	// Send hello.
	request := message.Request{
		Command:    "snapshot_get",
		Parameters: key_value.Empty().Set("block_timestamp", block_timestamp).Set("smartcontract_keys", smartcontract_keys),
	}
	parameters, err := socket.RequestRemoteService(&request)
	if err != nil {
		return nil, 0, fmt.Errorf("snapshot_get remote request: %w", err)
	}

	raw_logs, err := parameters.GetKeyValueList("logs")
	if err != nil {
		return nil, 0, fmt.Errorf("GetKeyValueList(`logs`): %w", err)
	}

	logs := make([]*Log, len(raw_logs))
	for i, raw := range raw_logs {
		log, err := NewFromMap(raw)
		if err != nil {
			return nil, 0, fmt.Errorf("raw_log[%d] converting to Log: %w", i, err)
		}
		logs[i] = log
	}

	block_timestamp_to, err := blockchain.NewTimestampFromKeyValueParameter(parameters)
	if err != nil {
		return nil, 0, fmt.Errorf("block timestamp from reply parameter: %w", err)
	}

	return logs, block_timestamp_to, nil
}

// Return list of logs for the transaction keys from the remote SDS Categorizer.
// For the transaction keys see
// github.com/blocklords/sds/categorizer/transaction.go TransactionKey()
func RemoteLogs(socket *remote.Socket, keys []string) ([]*Log, error) {
	request := message.Request{
		Command: "log_get_all",
		Parameters: map[string]interface{}{
			"keys": keys,
		},
	}
	raw_params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return nil, fmt.Errorf("log_get_all request: %w", err)
	}
	params := key_value.New(raw_params)

	raw_logs, err := params.GetKeyValueList("logs")
	if err != nil {
		return nil, fmt.Errorf("GetKeyValueList(`logs`): %w", err)
	}

	logs := make([]*Log, len(raw_logs))
	for i, raw := range raw_logs {
		log, err := NewFromMap(raw)
		if err != nil {
			return nil, fmt.Errorf("raw_log[%d] converting to Log: %w", i, err)
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

	raw_params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return "", nil, fmt.Errorf("parse remote request: %w", err)
	}

	params := key_value.New(raw_params)

	name, err := params.GetString("name")
	if err != nil {
		return "", nil, fmt.Errorf("parameter.GetString(`name`): %w", err)
	}
	args, err := params.GetKeyValue("args")
	if err != nil {
		return "", nil, fmt.Errorf("parameter.GetKeyValue(`args`): %w", err)
	}

	return name, args, nil
}
