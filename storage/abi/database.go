package abi

import (
	"fmt"

	"github.com/blocklords/sds/common/data_type"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/database/handler"
	"github.com/blocklords/sds/service/remote"
)

// Insert into database
//
// Implements common/data_type/database.Crud interface
func (a *Abi) Insert(db *remote.ClientSocket) error {
	request := handler.DatabaseQueryRequest{
		Fields:    []string{"abi_id", "body"},
		Tables:    []string{"abi"},
		Arguments: []interface{}{a.Id, data_type.AddJsonPrefix(a.Bytes)},
	}
	var reply handler.InsertReply

	err := handler.INSERT.Request(db, request, &reply)
	if err != nil {
		return fmt.Errorf("handler.INSERT.Request: %w", err)
	}
	return nil
}

// SelectAll abi from database
//
// Implements common/data_type/database.Crud interface
func (a *Abi) SelectAll(db_client *remote.ClientSocket, return_values interface{}) error {
	abis, ok := return_values.(*[]*Abi)
	if !ok {
		return fmt.Errorf("return_values.(*[]*Abi)")
	}

	request := handler.DatabaseQueryRequest{
		Fields: []string{"abi_id as id", "body as bytes"},
		Tables: []string{"storage_abi"},
	}
	var reply handler.SelectAllReply

	err := handler.SELECT_ALL.Request(db_client, request, &reply)
	if err != nil {
		return fmt.Errorf("handler.SELECT_ALL.Push: %w", err)
	}
	*abis = make([]*Abi, len(reply.Rows))

	// Loop through rows, using Scan to assign column data to struct fields.
	for i, raw := range reply.Rows {
		abi, err := New(raw)
		if err != nil {
			return fmt.Errorf("New Abi from database result: %w", err)
		}
		(*abis)[i] = abi
	}
	return_values = abis

	return err
}

// Not implemented common/data_type/database.Crud interface
//
// Returns an error
func (b *Abi) Select(_ *remote.ClientSocket) error {
	return fmt.Errorf("not implemented")
}

// Not implemented common/data_type/database.Crud interface
//
// Returns an error
func (b *Abi) SelectAllByCondition(_ *remote.ClientSocket, _ key_value.KeyValue, _ interface{}) error {
	return fmt.Errorf("not implemented")
}

// Not implemented common/data_type/database.Crud interface
//
// Returns an error
func (b *Abi) Exist(_ *remote.ClientSocket) bool {
	return false
}

// Not implemented common/data_type/database.Crud interface
//
// Returns an error
func (b *Abi) Update(_ *remote.ClientSocket, _ uint8) error {
	return fmt.Errorf("not implemented")
}
