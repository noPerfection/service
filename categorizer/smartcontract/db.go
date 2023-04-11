package smartcontract

import (
	"fmt"

	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/smartcontract_key"
	"github.com/blocklords/sds/db/handler"
)

// Set the block parameters in the database
func SaveBlockParameters(db *remote.ClientSocket, sm *Smartcontract) error {
	request := handler.DatabaseQueryRequest{
		Fields: []string{"block_number", "block_timestamp"},
		Tables: []string{"categorizer_smartcontract"},
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
func Exists(db *remote.ClientSocket, key smartcontract_key.Key) bool {
	request := handler.DatabaseQueryRequest{
		Tables: []string{"categorizer_smartcontract"},
		Where:  "network_id = ? AND address = ? ",
		Arguments: []interface{}{
			key.NetworkId,
			key.Address,
		},
	}
	var reply handler.ExistReply
	err := handler.EXIST.Request(db, request, &reply)
	if err != nil {
		return false
	}

	return reply.Exist
}

func Save(db *remote.ClientSocket, b *Smartcontract) error {
	request := handler.DatabaseQueryRequest{
		Fields: []string{"network_id", "address", "block_number", "block_timestamp"},
		Tables: []string{"categorizer_smartcontract"},
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
func Get(db *remote.ClientSocket, key smartcontract_key.Key) (*Smartcontract, error) {
	request := handler.DatabaseQueryRequest{
		Fields: []string{"block_number", "block_timestamp"},
		Tables: []string{"categorizer_smartcontract"},
		Where:  "network_id = ? AND address = ? ",
		Arguments: []interface{}{
			key.NetworkId,
			key.Address,
		},
	}
	var reply handler.SelectRowReply
	err := handler.SELECT_ROW.Request(db, request, &reply)
	if err != nil {
		return nil, fmt.Errorf("handler.SELECT_ROW.Request: %w", err)
	}

	block_number, err := blockchain.
		NewNumberFromKeyValueParameter(reply.Outputs)
	if err != nil {
		return nil, fmt.Errorf("blockchain.NewNumberFromKeyValueParameter(reply.Outputs): %w", err)
	}

	block_timestamp, err := blockchain.
		NewTimestampFromKeyValueParameter(reply.Outputs)
	if err != nil {
		return nil, fmt.Errorf("blockchain.NewTimestampFromKeyValueParameter(reply.Outputs): %w", err)
	}

	block_header, _ := blockchain.NewHeader(block_number.Value(), block_timestamp.Value())
	sm := Smartcontract{
		SmartcontractKey: key,
		BlockHeader:      block_header,
	}

	return &sm, nil
}

// Return all smartcontracts from database
func GetAll(db *remote.ClientSocket) ([]Smartcontract, error) {
	request := handler.DatabaseQueryRequest{
		Fields: []string{"network_id", "address", "block_number", "block_timestamp"},
		Tables: []string{"categorizer_smartcontract"},
	}
	var reply handler.SelectAllReply
	err := handler.SELECT_ALL.Request(db, request, &reply)
	if err != nil {
		return nil, fmt.Errorf("handler.SELECT_ALL.Request: %w", err)
	}

	smartcontracts := make([]Smartcontract, len(reply.Rows))
	// Loop through rows, using Scan to assign column data to struct fields.
	for i, raw := range reply.Rows {
		var sm = Smartcontract{
			SmartcontractKey: smartcontract_key.Key{},
			BlockHeader:      blockchain.BlockHeader{},
		}

		err := raw.ToInterface(&sm.SmartcontractKey)
		if err != nil {
			return nil, fmt.Errorf("failed to extract smartcontract key from database result: %w", err)
		}

		err = raw.ToInterface(&sm.BlockHeader)
		if err != nil {
			return nil, fmt.Errorf("failed to extract smartcontract key from database result: %w", err)
		}

		smartcontracts[i] = sm
	}
	return smartcontracts, err
}

// Returns list of the smartcontracts registered in the categorizer
func GetAllByNetworkId(db *remote.ClientSocket, network_id string) ([]Smartcontract, error) {
	request := handler.DatabaseQueryRequest{
		Fields:    []string{"network_id", "address", "block_number", "block_timestamp"},
		Tables:    []string{"categorizer_smartcontract"},
		Where:     " network_id = ? ",
		Arguments: []interface{}{network_id},
	}
	var reply handler.SelectAllReply
	err := handler.SELECT_ALL.Request(db, request, &reply)
	if err != nil {
		return nil, fmt.Errorf("handler.SELECT_ALL.Request: %w", err)
	}

	smartcontracts := make([]Smartcontract, len(reply.Rows))
	// Loop through rows, using Scan to assign column data to struct fields.
	for i, raw := range reply.Rows {
		var sm = Smartcontract{
			SmartcontractKey: smartcontract_key.Key{},
			BlockHeader:      blockchain.BlockHeader{},
		}

		err := raw.ToInterface(&sm.SmartcontractKey)
		if err != nil {
			return nil, fmt.Errorf("failed to extract smartcontract key from database result: %w", err)
		}

		err = raw.ToInterface(&sm.BlockHeader)
		if err != nil {
			return nil, fmt.Errorf("failed to extract smartcontract key from database result: %w", err)
		}

		smartcontracts[i] = sm
	}
	return smartcontracts, err
}
