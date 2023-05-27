package smartcontract

import (
	"fmt"

	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/smartcontract_key"
	"github.com/blocklords/sds/database/handler"
	"github.com/blocklords/sds/service/remote"
)

// Inserts the smartcontract into the database
//
// Implements common/data_type/database.Crud interface
func (sm *Smartcontract) Insert(db *remote.ClientSocket) error {
	request := handler.DatabaseQueryRequest{
		Fields: []string{"network_id",
			"address",
			"abi_id",
			"transaction_id",
			"transaction_index",
			"block_number",
			"block_timestamp",
			"deployer"},
		Tables: []string{"smartcontract"},
		Arguments: []interface{}{
			sm.SmartcontractKey.NetworkId,
			sm.SmartcontractKey.Address,
			sm.AbiId,
			sm.TransactionKey.Id,
			sm.TransactionKey.Index,
			sm.BlockHeader.Number,
			sm.BlockHeader.Timestamp,
			sm.Deployer,
		},
	}
	var reply handler.InsertReply

	err := handler.INSERT.Request(db, request, &reply)
	if err != nil {
		return fmt.Errorf("handler.INSERT.Request: %w", err)
	}
	return nil
}

// SelectAll smartcontracts from database
//
// Implements common/data_type/database.Crud interface
func (sm *Smartcontract) SelectAll(db *remote.ClientSocket, return_values interface{}) error {
	smartcontracts, ok := return_values.(*[]*Smartcontract)
	if !ok {
		return fmt.Errorf("return_values.(*[]*Smartcontract)")
	}

	request := handler.DatabaseQueryRequest{
		Fields: []string{
			"network_id",
			"address",
			"abi_id",
			"transaction_id",
			"transaction_index",
			"block_number",
			"block_timestamp",
			"deployer",
		},
		Tables: []string{"smartcontract"},
	}
	var reply handler.SelectAllReply

	err := handler.SELECT_ALL.Request(db, request, &reply)
	if err != nil {
		return fmt.Errorf("handler.SELECT_ALL.Request: %w", err)
	}

	*smartcontracts = make([]*Smartcontract, len(reply.Rows))

	// Loop through rows, using Scan to assign column data to struct fields.
	for i, raw := range reply.Rows {
		var sm = Smartcontract{
			SmartcontractKey: smartcontract_key.Key{},
			TransactionKey:   blockchain.TransactionKey{},
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

		err = raw.ToInterface(&sm.TransactionKey)
		if err != nil {
			return fmt.Errorf("raw.ToInterface(TransactionKey): %w", err)
		}

		deployer, err := raw.GetString("deployer")
		if err != nil {
			return fmt.Errorf("failed to extract deployer from database result: %w", err)
		}
		sm.Deployer = deployer

		abi_id, err := raw.GetString("abi_id")
		if err != nil {
			return fmt.Errorf("failed to extract abi id from database result: %w", err)
		}
		sm.AbiId = abi_id

		(*smartcontracts)[i] = &sm
	}

	return_values = smartcontracts

	return err
}

// Not implemented common/data_type/database.Crud interface
//
// Returns an error
func (sm *Smartcontract) Select(_ *remote.ClientSocket) error {
	return fmt.Errorf("not implemented")
}

// Not implemented common/data_type/database.Crud interface
//
// Returns an error
func (sm *Smartcontract) SelectAllByCondition(_ *remote.ClientSocket, _ key_value.KeyValue, _ interface{}) error {
	return fmt.Errorf("not implemented")
}

// Not implemented common/data_type/database.Crud interface
//
// Returns an error
func (sm *Smartcontract) Exist(_ *remote.ClientSocket) bool {
	return false
}

// Not implemented common/data_type/database.Crud interface
//
// Returns an error
func (sm *Smartcontract) Update(_ *remote.ClientSocket, _ uint8) error {
	return fmt.Errorf("not implemented")
}
