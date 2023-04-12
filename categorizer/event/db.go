package event

import (
	"fmt"
	"strconv"

	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/smartcontract_key"
	"github.com/blocklords/sds/db/handler"
)

// Insert the event log into database
//
// Implements common/data_type/database.Crud interface
func (t *Log) Insert(db_con *remote.ClientSocket) error {
	bytes, err := t.Parameters.ToBytes()
	if err != nil {
		return fmt.Errorf("event.Parameters.ToBytes %v: %w", t.Parameters, err)
	}

	request := handler.DatabaseQueryRequest{
		Fields: []string{
			"address",
			"transaction_id",
			"transaction_index",
			"network_id",
			"block_number",
			"block_timestamp",
			"log_index",
			"event_name",
			"event_parameters",
		},
		Tables: []string{"categorizer_event"},
		Arguments: []interface{}{
			t.SmartcontractKey.Address,
			t.TransactionKey.Id,
			t.TransactionKey.Index,
			t.SmartcontractKey.NetworkId,
			t.BlockHeader.Number,
			t.BlockHeader.Timestamp,
			t.Index,
			t.Name,
			data_type.AddJsonPrefix(bytes),
		},
	}
	var reply handler.InsertReply

	err = handler.INSERT.Request(db_con, request, &reply)
	if err != nil {
		return fmt.Errorf("handler.INSERT.Request: %w", err)
	}
	return nil
}

// Not implemented common/data_type/database.Crud interface
//
// Returns an error
func (b *Log) Select(_ *remote.ClientSocket) error {
	return fmt.Errorf("not implemented")
}

// Not implemented common/data_type/database.Crud interface
//
// Returns an error
func (b *Log) SelectAll(_ *remote.ClientSocket, _ interface{}) error {
	return fmt.Errorf("not implemented")
}

// Not implemented common/data_type/database.Crud interface
//
// Returns an error
func (b *Log) Exist(_ *remote.ClientSocket) bool {
	return false
}

// Not implemented common/data_type/database.Crud interface
//
// Returns an error
func (b *Log) Update(_ *remote.ClientSocket, _ uint8) error {
	return fmt.Errorf("not implemented")
}

// SelectAllByCondition the event log into database
//
// Implements common/data_type/database.Crud interface
//
// The condition is:
//
//   - "smartcontract_keys" = []smartcontract_key.Key
//     filter event logs for this smartcontracts
//
//   - "block_timestamp" = blockchain.Timestamp
//     event logs starting form this timestamp
//
//   - "limit" = uint64
//     maximum amount of event logs to return
func (l *Log) SelectAllByCondition(db_con *remote.ClientSocket, condition key_value.KeyValue, return_values interface{}) error {
	logs, ok := return_values.(*[]Log)
	if !ok {
		return fmt.Errorf("return_values.([]Log)")
	}

	smartcontract_keys, ok := condition["smartcontract_keys"].([]smartcontract_key.Key)
	if !ok {
		return fmt.Errorf("condition['smartcontract_keys'] is missing or invalid")
	}
	block_timestamp, ok := condition["block_timestamp"].(blockchain.Timestamp)
	if !ok {
		return fmt.Errorf("condition['block_timestamp'] is missing or invalid")
	}
	limit, err := condition.GetUint64("limit")
	if err != nil {
		return fmt.Errorf("condition['limit']: %w", err)
	}

	sm_amount := len(smartcontract_keys)

	if sm_amount == 0 {
		return nil
	}

	args := make([]interface{}, (sm_amount*2)+2)
	offset := 0
	args[offset] = block_timestamp.Value()
	offset++

	smartcontracts_clause := ""
	for i, sm := range smartcontract_keys {
		network_id := sm.NetworkId
		address := sm.Address

		smartcontracts_clause += "(network_id = ? AND address = ?) "
		if i < sm_amount-1 {
			smartcontracts_clause += " OR "
		}

		args[offset] = network_id
		offset++
		args[offset] = address
		offset++
	}
	args[offset] = strconv.FormatUint(limit, 10)

	request := handler.DatabaseQueryRequest{
		Fields: []string{
			"block_number",
			"block_timestamp",
			"transaction_id",
			"transaction_index",
			"log_index",
			"address",
			"network_id",
			"event_name",
			"event_parameters",
		},
		Tables:    []string{"categorizer_event"},
		Where:     ` block_timestamp >= ? AND ` + smartcontracts_clause + ` ORDER BY block_timestamp ASC LIMIT ? `,
		Arguments: args,
	}
	var reply handler.SelectAllReply
	err = handler.SELECT_ALL.Request(db_con, request, &reply)
	if err != nil {
		return fmt.Errorf("handler.SELECT_ALL.Request: %w", err)
	}

	*logs = make([]Log, len(reply.Rows))
	// Loop through rows, using Scan to assign column data to struct fields.
	for i, raw := range reply.Rows {
		var s Log
		var key smartcontract_key.Key
		var transaction_key blockchain.TransactionKey
		var block_header blockchain.BlockHeader

		err := raw.ToInterface(&key)
		if err != nil {
			return fmt.Errorf("failed to extract smartcontract key from database result: %w", err)
		}

		err = raw.ToInterface(&transaction_key)
		if err != nil {
			return fmt.Errorf("failed to extract transaction key from database result: %w", err)
		}

		err = raw.ToInterface(&block_header)
		if err != nil {
			return fmt.Errorf("failed to extract block_header header from database result: %w", err)
		}

		s.BlockHeader = block_header
		s.TransactionKey = transaction_key
		s.SmartcontractKey = key

		name, err := raw.GetString("event_name")
		if err != nil {
			return fmt.Errorf("failed to extract event_name from database result: %w", err)
		}
		s.Name = name

		index, err := raw.GetUint64("log_index")
		if err != nil {
			return fmt.Errorf("failed to extract event_index from database result: %w", err)
		}
		s.Index = uint(index)

		parameters_base, err := raw.GetString("event_parameters")
		if err != nil {
			return fmt.Errorf("failed to extract event_parameters from database result: %w", err)
		}
		raw_parameters := data_type.DecodeJsonPrefixed(parameters_base)
		if len(raw_parameters) == 0 {
			return fmt.Errorf("data_type.DecodeJsonPrefixed %s: %w", parameters_base, err)
		}

		parameters, err := key_value.NewFromString(string(raw_parameters))
		if err != nil {
			return fmt.Errorf("key_value.NewFromString(event_parameters): %w", err)
		}
		s.Parameters = parameters

		(*logs)[i] = s
	}
	return_values = logs

	return err
}
