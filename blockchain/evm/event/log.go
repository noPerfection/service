package event

import (
	"encoding/hex"

	"github.com/blocklords/gosds/blockchain/event"

	eth_types "github.com/ethereum/go-ethereum/core/types"
)

// Converts the ethereum's log to SeascapeSDS Spaghetti Log type
func NewSpaghettiLog(network_id string, block_timestamp uint64, raw_log *eth_types.Log) *event.Log {
	topics := make([]string, len(raw_log.Topics))
	for i, topic := range raw_log.Topics {
		topics[i] = topic.Hex()
	}

	return &event.Log{
		NetworkId:      network_id,
		BlockNumber:    raw_log.BlockHash.Big().Uint64(),
		BlockTimestamp: block_timestamp,
		Txid:           raw_log.TxHash.Hex(),
		LogIndex:       raw_log.Index,
		Data:           hex.EncodeToString(raw_log.Data),
		Address:        raw_log.Address.Hex(),
		Topics:         topics,
	}
}

// Converts the ethereum's log to SeascapeSDS Spaghetti Log type
func NewSpaghettiLogs(network_id string, block_timestamp uint64, raw_logs []eth_types.Log) []*event.Log {
	logs := make([]*event.Log, len(raw_logs))
	for i, raw := range raw_logs {
		log := NewSpaghettiLog(network_id, block_timestamp, &raw)
		logs[i] = log
	}

	return logs
}
