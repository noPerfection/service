package event

import (
	"encoding/hex"

	"github.com/blocklords/sds/blockchain/event"
	"github.com/blocklords/sds/blockchain/transaction"
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/static/smartcontract/key"

	eth_types "github.com/ethereum/go-ethereum/core/types"
)

// Converts the ethereum's log to SeascapeSDS Spaghetti Log type
func NewSpaghettiLog(network_id string, block_timestamp blockchain.Timestamp, raw_log *eth_types.Log) *event.RawLog {
	topics := make([]string, len(raw_log.Topics))
	for i, topic := range raw_log.Topics {
		topics[i] = topic.Hex()
	}

	key := key.New(network_id, raw_log.Address.Hex())
	block := blockchain.New(raw_log.BlockNumber, uint64(block_timestamp))
	tx_key := blockchain.TransactionKey{
		Id:    raw_log.TxHash.Hex(),
		Index: raw_log.TxIndex,
	}

	transaction := transaction.RawTransaction{
		Key:            key,
		Block:          block,
		TransactionKey: tx_key,
		Data:           "",
		Value:          0,
	}

	return &event.RawLog{
		Transaction: transaction,
		LogIndex:    raw_log.Index,
		Data:        hex.EncodeToString(raw_log.Data),
		Topics:      topics,
	}
}

// Converts the ethereum's log to SeascapeSDS Spaghetti Log type
func NewSpaghettiLogs(network_id string, block_timestamp blockchain.Timestamp, raw_logs []eth_types.Log) []*event.RawLog {
	logs := make([]*event.RawLog, len(raw_logs))
	for i, raw := range raw_logs {
		log := NewSpaghettiLog(network_id, block_timestamp, &raw)
		logs[i] = log
	}

	return logs
}
