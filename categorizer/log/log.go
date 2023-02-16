/*Categorized log containing log name and output parameters*/
package log

import (
	"github.com/blocklords/gosds/categorizer/smartcontract"
	spaghetti_log "github.com/blocklords/gosds/spaghetti/log"
)

// The Smartcontract Event Log
type Log struct {
	ID             uint64 // ID in the database
	NetworkId      string // Network ID
	Txid           string // Transaction ID where it occured
	BlockNumber    uint64
	BlockTimestamp uint64
	LogIndex       uint                   // Log index in the block
	Address        string                 // Smartcontract address
	Log            string                 // Event log name
	Output         map[string]interface{} // Event log parameters
}

// Add the metadata such as transaction id and log index from spaghetti data
func (log *Log) AddMetadata(spaghetti_log *spaghetti_log.Log) *Log {
	log.Txid = spaghetti_log.Txid
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

// Convert to the Map[string]interface
func (log *Log) ToJSON() map[string]interface{} {
	return map[string]interface{}{
		"network_id":      log.NetworkId,
		"txid":            log.Txid,
		"block_timestamp": log.BlockTimestamp,
		"block_number":    log.BlockNumber,
		"log_index":       log.LogIndex,
		"address":         log.Address,
		"log":             log.Log,
		"output":          log.Output,
	}
}
