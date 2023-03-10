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
type RawLog struct {
	Transaction transaction.RawTransaction `json:"transaction"`
	LogIndex    uint                       `json:"log_index"`            // index
	Data        string                     `json:"log_data"`             // datatext data type
	Topics      []string                   `json:"log_topics,omitempty"` // topics
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
func (b *RawLog) TopicRaw() []byte {
	byt, err := json.Marshal(b.Topics)
	if err != nil {
		return []byte{}
	}

	return byt
}

// Converts the byte series into the topic list
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
		if strings.EqualFold(address, log.Transaction.Key.Address) {
			logs = append(logs, log)
		}
	}

	return logs
}

// Get the most recent block parameters
// returns block_number and block_timestamp
func RecentBlock(all_logs []*RawLog) blockchain.Block {
	block := blockchain.NewBlock(0, 0)

	for _, log := range all_logs {
		if log.Transaction.Block.Number > block.Number {
			block = log.Transaction.Block
		}
	}

	return block
}
