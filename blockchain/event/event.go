/*Spaghetti transaction without method name and without clear input parameters*/
package event

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/blocklords/sds/blockchain/transaction"
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type/key_value"
)

// Blockchain agnostic Event Log for smartcontract
//
// Based on the EVM blockchain's event log. However,
// it's generalized for all blockchain's types.
type RawLog struct {
	// Transaction where the log was occured
	Transaction transaction.RawTransaction `json:"transaction"`
	// Index of the log in the transaction's log list.
	Index uint `json:"log_index"` // index
	// Raw data of the log
	Data string `json:"log_data"` // datatext data type
	// Indexed data of the log
	Topics []string `json:"log_topics,omitempty"` // topics
}

// JSON string representation of the spaghetti.Log
func (l *RawLog) ToString() (string, error) {
	kv, err := key_value.NewFromInterface(l)
	if err != nil {
		return "", fmt.Errorf("failed to serialize spaghetti log to intermediate key-value %v: %v", l, err)
	}

	bytes, err := kv.ToBytes()
	if err != nil {
		return "", fmt.Errorf("failed to serialize intermediate key-value to string %v: %v", l, err)
	}

	return string(bytes), nil
}

// Serielizes the Log.Topics into the byte array
// Reverse of log.ParseTopics()
func (b *RawLog) TopicRaw() []byte {
	byt, err := json.Marshal(b.Topics)
	if err != nil {
		return []byte{}
	}

	return byt
}

// Converts the byte array into the topic list.
// Reverse of log.TopicRaw()
func (b *RawLog) ParseTopics(raw []byte) error {
	var topics []string
	err := json.Unmarshal(raw, &topics)
	if err != nil {
		return fmt.Errorf("json.deserialize: %w", err)
	}
	b.Topics = topics

	return nil
}

// Get the slice of logs filtered by the smartcontract address
func FilterByAddress(all_logs []*RawLog, address string) []*RawLog {
	logs := make([]*RawLog, 0)

	for _, log := range all_logs {
		if strings.EqualFold(address, log.Transaction.SmartcontractKey.Address) {
			logs = append(logs, log)
		}
	}

	return logs
}

// Get the most recent block's header
// within the log list.
func RecentBlock(all_logs []RawLog) blockchain.BlockHeader {
	block := blockchain.NewHeader(0, 0)

	for _, log := range all_logs {
		if log.Transaction.BlockHeader.Number > block.Number {
			block = log.Transaction.BlockHeader
		}
	}

	return block
}
