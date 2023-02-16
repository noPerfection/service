/*Spaghetti transaction without method name and without clear input parameters*/
package log

import (
	"encoding/json"
)

type Log struct {
	NetworkId      string
	Txid           string // txId column
	BlockNumber    uint64
	BlockTimestamp uint64
	LogIndex       uint
	Data           string // text data type
	Topics         []string
	Address        string
}

// JSON representation of the spaghetti.Log
func (b *Log) ToJSON() map[string]interface{} {
	return map[string]interface{}{
		"network_id":      b.NetworkId,
		"txid":            b.Txid,
		"block_timestamp": b.BlockTimestamp,
		"block_number":    b.BlockNumber,
		"log_index":       b.LogIndex,
		"data":            b.Data,
		"topics":          b.Topics,
		"address":         b.Address,
	}
}

// JSON string representation of the spaghetti.Log
func (b *Log) ToString() string {
	interfaces := b.ToJSON()
	byt, err := json.Marshal(interfaces)
	if err != nil {
		return ""
	}

	return string(byt)
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
