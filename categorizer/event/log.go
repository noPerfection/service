/*Categorized log containing log name and output parameters*/
package event

import (
	spaghetti_log "github.com/blocklords/sds/blockchain/event"
	"github.com/blocklords/sds/categorizer/smartcontract"
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/smartcontract_key"
)

// The Smartcontract Event Log
type Log struct {
	SmartcontractKey smartcontract_key.Key     `json:"smartcontract_key`
	TransactionKey   blockchain.TransactionKey `json:"transaction_key`
	Block            blockchain.Block          `json:"block"`
	LogIndex         uint                      `json:"log_index"`        // Log index in the block
	Name             string                    `json:"event_name"`       // Log                 // Event log name
	Parameters       key_value.KeyValue        `json:"event_parameters"` // Event log parameters
}

// Add the metadata such as transaction id and log index from spaghetti data
func (log *Log) AddMetadata(spaghetti_log *spaghetti_log.RawLog) *Log {
	log.TransactionKey = spaghetti_log.Transaction.TransactionKey
	log.Block = spaghetti_log.Transaction.Block
	log.LogIndex = spaghetti_log.LogIndex
	return log
}

// add the smartcontract to which this log belongs too using categorizer.Smartcontract
func (log *Log) AddSmartcontractData(smartcontract *smartcontract.Smartcontract) *Log {
	log.SmartcontractKey = smartcontract.Key
	return log
}
