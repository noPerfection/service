package log

import (
	"errors"

	"github.com/blocklords/gosds/common/data_type/key_value"
)

// Convert the JSON into spaghetti.Log
func New(parameters key_value.KeyValue) (*Log, error) {
	topics, err := parameters.GetStringList("topics")
	if err != nil {
		return nil, err
	}
	network_id, err := parameters.GetString("network_id")
	if err != nil {
		return nil, err
	}
	txid, err := parameters.GetString("txid")
	if err != nil {
		return nil, err
	}
	log_index, err := parameters.GetUint64("log_index")
	if err != nil {
		return nil, err
	}
	data, err := parameters.GetString("data")
	if err != nil {
		return nil, err
	}
	address, err := parameters.GetString("address")
	if err != nil {
		return nil, err
	}

	block_timestamp, err := parameters.GetUint64("block_timestamp")
	if err != nil {
		return nil, err
	}
	block_number, err := parameters.GetUint64("block_number")
	if err != nil {
		return nil, err
	}

	return &Log{
		NetworkId:      network_id,
		Address:        address,
		Txid:           txid,
		BlockNumber:    block_number,
		BlockTimestamp: block_timestamp,
		LogIndex:       uint(log_index),
		Data:           data,
		Topics:         topics,
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
			return nil, err
		}
		logs[i] = l
	}
	return logs, nil
}
