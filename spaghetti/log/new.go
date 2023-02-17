package log

import (
	"encoding/hex"
	"errors"

	"github.com/blocklords/gosds/app/remote/message"
	eth_types "github.com/ethereum/go-ethereum/core/types"
)

// Converts the ethereum's log to SeascapeSDS Spaghetti Log type
func NewFromRawLog(network_id string, log *eth_types.Log) (*Log, error) {
	topics := make([]string, len(log.Topics))
	for i, topic := range log.Topics {
		topics[i] = topic.Hex()
	}

	return &Log{
		NetworkId: network_id,
		Txid:      log.TxHash.Hex(),
		LogIndex:  log.Index,
		Data:      hex.EncodeToString(log.Data),
		Address:   log.Address.Hex(),
		Topics:    topics,
	}, nil
}

// Convert the JSON into spaghetti.Log
func New(parameters map[string]interface{}) (*Log, error) {
	topics, err := message.GetStringList(parameters, "topics")
	if err != nil {
		return nil, err
	}
	network_id, err := message.GetString(parameters, "network_id")
	if err != nil {
		return nil, err
	}
	txid, err := message.GetString(parameters, "txid")
	if err != nil {
		return nil, err
	}
	log_index, err := message.GetUint64(parameters, "log_index")
	if err != nil {
		return nil, err
	}
	data, err := message.GetString(parameters, "data")
	if err != nil {
		return nil, err
	}
	address, err := message.GetString(parameters, "address")
	if err != nil {
		return nil, err
	}

	block_timestamp, err := message.GetUint64(parameters, "block_timestamp")
	if err != nil {
		return nil, err
	}
	block_number, err := message.GetUint64(parameters, "block_number")
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
