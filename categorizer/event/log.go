/*Categorized log containing log name and output parameters*/
package event

import (
	spaghetti_log "github.com/blocklords/sds/blockchain/event"
	"github.com/blocklords/sds/categorizer/smartcontract"
)

// The Smartcontract Event Log
type Log struct {
	NetworkId        string                 `json:"network_id"`        // Network ID
	TransactionId    string                 `json:"transaction_id"`    // Transaction ID where it occured
	TransactionIndex uint                   `json:"transaction_index"` // Transaction index
	BlockNumber      uint64                 `json:"block_number"`
	BlockTimestamp   uint64                 `json:"block_timestamp"`
	LogIndex         uint                   `json:"log_index"` // Log index in the block
	Address          string                 `json:"address"`   // Address                 // Smartcontract address
	Log              string                 `json:"log"`       // Log                 // Event log name
	Output           map[string]interface{} `json:"output"`    // Event log parameters
}

// Add the metadata such as transaction id and log index from spaghetti data
func (log *Log) AddMetadata(spaghetti_log *spaghetti_log.Log) *Log {
	log.TransactionId = spaghetti_log.TransactionId
	log.TransactionIndex = spaghetti_log.TransactionIndex
	log.BlockNumber = spaghetti_log.BlockNumber
	log.BlockTimestamp = spaghetti_log.BlockTimestamp
	log.LogIndex = spaghetti_log.LogIndex
	return log
}

// add the smartcontract to which this log belongs too using categorizer.Smartcontract
func (log *Log) AddSmartcontractData(smartcontract *smartcontract.Smartcontract) *Log {
	log.NetworkId = smartcontract.NetworkId
	log.Address = smartcontract.Address
	return log
}
