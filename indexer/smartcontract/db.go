package smartcontract

import (
	"fmt"

	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/smartcontract_key"
	"github.com/blocklords/sds/db/handler"
	"github.com/blocklords/sds/service/remote"
)

const UPDATE_BLOCK_HEADER uint8 = 1

// Set the block parameters in the database
//
// Implements common/data_type/database.Crud interface
func (sm *Smartcontract) Update(db *remote.ClientSocket, _ uint8) error {
	request := handler.DatabaseQueryRequest{
		Fields: []string{"block_number", "block_timestamp"},
		Tables: []string{"indexer_smartcontract"},
		Where:  " network_id = ? AND address = ? ",
		Arguments: []interface{}{
			sm.BlockHeader.Number,
			sm.BlockHeader.Timestamp,
			sm.SmartcontractKey.NetworkId,
			sm.SmartcontractKey.Address,
		},
	}
	var reply handler.UpdateReply
	err := handler.UPDATE.Request(db, request, &reply)
	if err != nil {
		return fmt.Errorf("handler.UPDATE.Request: %w", err)
	}
	return nil
}

// Exists checks whether the smartcontract exists or not by it's smartcontract key
//
// Note that if database layer returns an error, it won't return an error.
//
// Implements common/data_type/database.Crud interface
func (sm *Smartcontract) Exist(db *remote.ClientSocket) bool {
	request := handler.DatabaseQueryRequest{
		Tables: []string{"indexer_smartcontract"},
		Where:  "network_id = ? AND address = ? ",
		Arguments: []interface{}{
			sm.SmartcontractKey.NetworkId,
			sm.SmartcontractKey.Address,
		},
	}
	var reply handler.ExistReply
	err := handler.EXIST.Request(db, request, &reply)
	if err != nil {
		return false
	}

	return reply.Exist
}

// Insert the data into database
//
// Implements common/data_type/database.Crud interface
func (b *Smartcontract) Insert(db *remote.ClientSocket) error {
	request := handler.DatabaseQueryRequest{
		Fields: []string{"network_id", "address", "block_number", "block_timestamp"},
		Tables: []string{"indexer_smartcontract"},
		Arguments: []interface{}{
			b.SmartcontractKey.NetworkId,
			b.SmartcontractKey.Address,
			b.BlockHeader.Number,
			b.BlockHeader.Timestamp,
		},
	}
	var reply handler.InsertReply

	err := handler.INSERT.Request(db, request, &reply)
	if err != nil {
		return fmt.Errorf("handler.INSERT.Request: %w", err)
	}
	return nil
}

// Return the single smartcontract from database
//
// Implements common/data_type/database.Crud interface
func (b *Smartcontract) Select(db *remote.ClientSocket) error {
	request := handler.DatabaseQueryRequest{
		Fields: []string{"block_number", "block_timestamp"},
		Tables: []string{"indexer_smartcontract"},
		Where:  "network_id = ? AND address = ? ",
		Arguments: []interface{}{
			b.SmartcontractKey.NetworkId,
			b.SmartcontractKey.Address,
		},
	}
	var reply handler.SelectRowReply
	err := handler.SELECT_ROW.Request(db, request, &reply)
	if err != nil {
		return fmt.Errorf("handler.SELECT_ROW.Request: %w", err)
	}

	block_number, err := blockchain.
		NewNumberFromKeyValueParameter(reply.Outputs)
	if err != nil {
		return fmt.Errorf("blockchain.NewNumberFromKeyValueParameter(reply.Outputs): %w", err)
	}

	block_timestamp, err := blockchain.
		NewTimestampFromKeyValueParameter(reply.Outputs)
	if err != nil {
		return fmt.Errorf("blockchain.NewTimestampFromKeyValueParameter(reply.Outputs): %w", err)
	}

	block_header, _ := blockchain.NewHeader(block_number.Value(), block_timestamp.Value())
	b.BlockHeader = block_header
	return nil
}

// Return all smartcontracts from database
//
// Implements common/data_type/database.Crud interface
func (b *Smartcontract) SelectAll(db *remote.ClientSocket, return_values interface{}) error {
	request := handler.DatabaseQueryRequest{
		Fields: []string{"network_id", "address", "block_number", "block_timestamp"},
		Tables: []string{"indexer_smartcontract"},
	}
	var reply handler.SelectAllReply
	err := handler.SELECT_ALL.Request(db, request, &reply)
	if err != nil {
		return fmt.Errorf("handler.SELECT_ALL.Request: %w", err)
	}

	raw_smartcontracts, ok := return_values.(*[]Smartcontract)
	if !ok {
		return fmt.Errorf("return_values.([]Smartcontract): %w", err)
	}
	*raw_smartcontracts = make([]Smartcontract, len(reply.Rows))

	// Loop through rows, using Scan to assign column data to struct fields.
	for i, raw := range reply.Rows {
		var sm = Smartcontract{
			SmartcontractKey: smartcontract_key.Key{},
			BlockHeader:      blockchain.BlockHeader{},
		}

		err := raw.ToInterface(&sm.SmartcontractKey)
		if err != nil {
			return fmt.Errorf("raw.ToInterface(SmartcontractKey): %w", err)
		}

		err = raw.ToInterface(&sm.BlockHeader)
		if err != nil {
			return fmt.Errorf("raw.ToInterface(BlockHeader): %w", err)
		}

		(*raw_smartcontracts)[i] = sm
	}
	return_values = raw_smartcontracts

	return err
}

// Returns list of the smartcontracts registered in the indexer
//
// Implements common/data_type/database.Crud interface
func (b *Smartcontract) SelectAllByCondition(db *remote.ClientSocket, condition key_value.KeyValue, return_values interface{}) error {
	network_id, err := condition.GetString("network_id")
	if err != nil {
		return fmt.Errorf("missing network_id condition value")
	}
	request := handler.DatabaseQueryRequest{
		Fields:    []string{"network_id", "address", "block_number", "block_timestamp"},
		Tables:    []string{"indexer_smartcontract"},
		Where:     " network_id = ? ",
		Arguments: []interface{}{network_id},
	}
	var reply handler.SelectAllReply
	err = handler.SELECT_ALL.Request(db, request, &reply)
	if err != nil {
		return fmt.Errorf("handler.SELECT_ALL.Request: %w", err)
	}

	raw_smartcontracts, ok := return_values.(*[]Smartcontract)
	if !ok {
		return fmt.Errorf("return_values.([]Smartcontract): %w", err)
	}
	*raw_smartcontracts = make([]Smartcontract, len(reply.Rows))

	// Loop through rows, using Scan to assign column data to struct fields.
	for i, raw := range reply.Rows {
		var sm = Smartcontract{
			SmartcontractKey: smartcontract_key.Key{},
			BlockHeader:      blockchain.BlockHeader{},
		}

		err := raw.ToInterface(&sm.SmartcontractKey)
		if err != nil {
			return fmt.Errorf("raw.ToInterface(SmartcontractKey): %w", err)
		}

		err = raw.ToInterface(&sm.BlockHeader)
		if err != nil {
			return fmt.Errorf("raw.ToInterface(BlockHeader): %w", err)
		}

		(*raw_smartcontracts)[i] = sm
	}
	return_values = raw_smartcontracts

	return err
}
