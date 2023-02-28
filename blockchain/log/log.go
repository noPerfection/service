/*Spaghetti transaction without method name and without clear input parameters*/
package log

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/blocklords/gosds/common/data_type/key_value"
)

type Log struct {
	NetworkId      string   `json:"network_id"`
	Txid           string   `json:"txid"`             // txId column
	BlockNumber    uint64   `json:"block_number"`     // block
	BlockTimestamp uint64   `json:"block_timestamp"`  // block
	LogIndex       uint     `json:"log_index"`        // index
	Data           string   `json:"data"`             // datatext data type
	Topics         []string `json:"topics,omitempty"` // topics
	Address        string   `json:"address"`          // address
}

// JSON string representation of the spaghetti.Log
func (l *Log) ToString() (string, error) {
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
func (b *Log) TopicRaw() []byte {
	byt, err := json.Marshal(b.Topics)
	if err != nil {
		return []byte{}
	}

	return byt
}

// Converts the byte series into the topic list
func (b *Log) ParseTopics(raw []byte) error {
	var topics []string
	err := json.Unmarshal(raw, &topics)
	if err != nil {
		return err
	}
	b.Topics = topics

	return nil
}

// Get the slice of logs filtered by the smartcontract address
func FilterByAddress(all_logs []*Log, address string) []*Log {
	logs := make([]*Log, 0)

	for _, log := range all_logs {
		if strings.EqualFold(address, log.Address) {
			logs = append(logs, log)
		}
	}

	return logs
}

func RecentBlock(all_logs []*Log) (uint64, uint64) {
	block_number := uint64(0)
	block_timestamp := uint64(0)

	for _, log := range all_logs {
		if log.BlockNumber > block_number {
			block_number = log.BlockNumber
			block_timestamp = log.BlockTimestamp
		}
	}

	return block_number, block_timestamp
}
