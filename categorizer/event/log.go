/*Categorized log containing log name and output parameters*/
package event

import (
	spaghetti_log "github.com/blocklords/sds/blockchain/event"
	"github.com/blocklords/sds/categorizer/smartcontract"
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/smartcontract_key"
)

// The Decoded smartcontract event log
type Log struct {
	SmartcontractKey smartcontract_key.Key     `json:"smartcontract_key"`
	TransactionKey   blockchain.TransactionKey `json:"transaction_key"`
	BlockHeader      blockchain.BlockHeader    `json:"block_header"`
	Index            uint                      `json:"log_index"`      // Log index in the block
	Name             string                    `json:"log_name"`       // Log                 // Event log name
	Parameters       key_value.KeyValue        `json:"log_parameters"` // Event log parameters
}

// Add the metadata such as transaction id, block header and log index from raw event log
func (log *Log) AddMetadata(spaghetti_log *spaghetti_log.RawLog) *Log {
	log.TransactionKey = spaghetti_log.Transaction.TransactionKey
	log.BlockHeader = spaghetti_log.Transaction.BlockHeader
	log.Index = spaghetti_log.Index
	return log
}

// Add the smartcontract that's associated with this event log
func (log *Log) AddSmartcontractData(smartcontract *smartcontract.Smartcontract) *Log {
	log.SmartcontractKey = smartcontract.SmartcontractKey
	return log
}
