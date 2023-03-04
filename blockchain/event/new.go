package event

import (
	"errors"
	"fmt"

	"github.com/blocklords/gosds/common/data_type/key_value"
)

// Convert the JSON into spaghetti.Log
// https://docs.soliditylang.org/en/v0.8.4/abi-spec.html?highlight=anonymous#json
func New(parameters key_value.KeyValue) (*Log, error) {
	topics, err := parameters.GetStringList("topics")
	if err != nil {
		topics = []string{}
	}
	network_id, err := parameters.GetString("network_id")
	if err != nil {
		return nil, fmt.Errorf("GetString(`network_id`): %w", err)
	}
	txid, err := parameters.GetString("transaction_id")
	if err != nil {
		return nil, fmt.Errorf("GetString(`transaction_id`): %w", err)
	}
	log_index, err := parameters.GetUint64("log_index")
	if err != nil {
		return nil, fmt.Errorf("GetUint64(`log_index`): %w", err)
	}
	data, err := parameters.GetString("data")
	if err != nil {
		return nil, fmt.Errorf("GetString(`data`): %w", err)
	}
	address, err := parameters.GetString("address")
	if err != nil {
		return nil, fmt.Errorf("GetString(`address`): %w", err)
	}

	block_timestamp, err := parameters.GetUint64("block_timestamp")
	if err != nil {
		return nil, fmt.Errorf("GetUint64(`block_timestamp`): %w", err)
	}
	block_number, err := parameters.GetUint64("block_number")
	if err != nil {
		return nil, fmt.Errorf("GetUint64(`block_number`): %w", err)
	}
	transaction_index, err := parameters.GetUint64("transaction_index")
	if err != nil {
		return nil, fmt.Errorf("GetUint64(`transaction_index`): %w", err)
	}

	return &Log{
		NetworkId:        network_id,
		Address:          address,
		TransactionId:    txid,
		TransactionIndex: uint(transaction_index),
		BlockNumber:      block_number,
		BlockTimestamp:   block_timestamp,
		LogIndex:         uint(log_index),
		Data:             data,
		Topics:           topics,
	}, nil
}

// Parse list of Logs into array of spaghetti.Log
func NewLogs(raw_logs []interface{}) ([]*Log, error) {
	logs := make([]*Log, len(raw_logs))
	for i, raw := range raw_logs {
		if raw == nil {
			continue
		}
		log_map, ok := raw.(map[string]interface{})
		if !ok {
			return nil, errors.New("the log is not a map")
		}
		l, err := New(log_map)
		if err != nil {
			return nil, fmt.Errorf("log[%d] converting to Log: %w", i, err)
		}
		logs[i] = l
	}
	return logs, nil
}
