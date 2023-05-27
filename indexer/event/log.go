// Package event defines the decoded smartcontract event
package event

import (
	"fmt"

	blockchain_log "github.com/blocklords/sds/blockchain/event"
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/smartcontract_key"
	"github.com/blocklords/sds/indexer/smartcontract"
)

// Log of smartcontract event after decoding
type Log struct {
	SmartcontractKey smartcontract_key.Key     `json:"smartcontract_key"`
	TransactionKey   blockchain.TransactionKey `json:"transaction_key"`
	BlockHeader      blockchain.BlockHeader    `json:"block_header"`
	Index            uint                      `json:"log_index"`        // Log index in the block
	Name             string                    `json:"event_name"`       // Event name
	Parameters       key_value.KeyValue        `json:"event_parameters"` // Decoded log parameters
}

// Add the metadata such as transaction id, block header and log index from raw event log
func (log *Log) AddMetadata(blockchain_log *blockchain_log.RawLog) *Log {
	log.TransactionKey = blockchain_log.Transaction.TransactionKey
	log.BlockHeader = blockchain_log.Transaction.BlockHeader
	log.Index = blockchain_log.Index
	return log
}

// Add the smartcontract that's associated with this event log
func (log *Log) AddSmartcontractData(smartcontract *smartcontract.Smartcontract) *Log {
	log.SmartcontractKey = smartcontract.SmartcontractKey
	return log
}

// Validate the Log parameters.
//
// Such that none of the string fields
// and nested struct fields can not be empty
func (log *Log) Validate() error {
	if log == nil {
		return fmt.Errorf("invalid log")
	}

	if len(log.Name) == 0 {
		return fmt.Errorf("the 'Name' field is empty")
	}
	if log.Parameters == nil {
		return fmt.Errorf("the 'Parameters' is nil")
	}
	if err := log.SmartcontractKey.Validate(); err != nil {
		return fmt.Errorf("log.SmartcontractKey.Validate: %w", err)
	}
	if err := log.TransactionKey.Validate(); err != nil {
		return fmt.Errorf("log.TransactionKey.Validate: %w", err)
	}
	if err := log.BlockHeader.Validate(); err != nil {
		return fmt.Errorf("log.BlockHeader.Validate: %w", err)
	}

	return nil
}
