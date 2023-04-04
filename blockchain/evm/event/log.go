package event

import (
	"encoding/hex"
	"fmt"

	"github.com/blocklords/sds/blockchain/event"
	"github.com/blocklords/sds/blockchain/transaction"
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/smartcontract_key"

	eth_types "github.com/ethereum/go-ethereum/core/types"
)

// Converts the ethereum's log to SeascapeSDS Spaghetti Log type
func NewSpaghettiLog(network_id string, block_timestamp blockchain.Timestamp, raw_log *eth_types.Log) (*event.RawLog, error) {
	topics := make([]string, len(raw_log.Topics))
	for i, topic := range raw_log.Topics {
		topics[i] = topic.Hex()
	}

	key, err := smartcontract_key.New(network_id, raw_log.Address.Hex())
	if err != nil {
		return nil, fmt.Errorf("smartcontract_key.New: %w", err)
	}
	block, err := blockchain.NewHeader(raw_log.BlockNumber, uint64(block_timestamp))
	if err != nil {
		return nil, fmt.Errorf("blockchain.NewHeader: %w", err)
	}
	tx_key := blockchain.TransactionKey{
		Id:    raw_log.TxHash.Hex(),
		Index: raw_log.TxIndex,
	}

	transaction := transaction.RawTransaction{
		SmartcontractKey: key,
		BlockHeader:      block,
		TransactionKey:   tx_key,
		Data:             "",
		Value:            0,
	}

	return &event.RawLog{
		Transaction: transaction,
		Index:       raw_log.Index,
		Data:        hex.EncodeToString(raw_log.Data),
		Topics:      topics,
	}, nil
}

// Converts the ethereum's log to SeascapeSDS Spaghetti Log type
func NewSpaghettiLogs(network_id string, block_timestamp blockchain.Timestamp, raw_logs []eth_types.Log) ([]*event.RawLog, error) {
	logs := make([]*event.RawLog, len(raw_logs))
	for i, raw := range raw_logs {
		log, err := NewSpaghettiLog(network_id, block_timestamp, &raw)
		if err != nil {
			return nil, fmt.Errorf("the %d log creation error: %v", i, err)
		}
		logs[i] = log
	}

	return logs, nil
}
