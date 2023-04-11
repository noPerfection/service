package event

import (
	"fmt"

	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type"
	"github.com/blocklords/sds/common/smartcontract_key"
	"github.com/blocklords/sds/db/handler"
)

func Save(db_con *remote.ClientSocket, t *Log) error {
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
			data_type.SerializeBytes(bytes),
		},
	}
	var reply handler.InsertReply

	err = handler.INSERT.Request(db_con, request, &reply)
	if err != nil {
		return fmt.Errorf("handler.INSERT.Request: %w", err)
	}
	return nil
}

// returns list of logs for smartcontracts
func GetLogsFromDb(
	db_con *remote.ClientSocket,
	smartcontracts []smartcontract_key.Key,
	block_timestamp blockchain.Timestamp,
	limit uint64) ([]Log, error) {
	sm_amount := len(smartcontracts)

	if sm_amount == 0 {
		return []Log{}, nil
	}

	args := make([]interface{}, (sm_amount*2)+2)
	offset := 0
	args[offset] = block_timestamp
	offset++

	smartcontracts_clause := ""
	for i, sm := range smartcontracts {
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
	args[offset] = limit

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
	err := handler.SELECT_ALL.Request(db_con, request, &reply)
	if err != nil {
		return nil, fmt.Errorf("handler.SELECT_ALL.Request: %w", err)
	}

	logs := make([]Log, len(reply.Rows))
	// Loop through rows, using Scan to assign column data to struct fields.
	for i, raw := range reply.Rows {
		var s Log
		var output_bytes []byte

		err := raw.ToInterface(&s)
		if err != nil {
			return nil, fmt.Errorf("failed to extract smartcontract key from database result: %w", err)
		}

		err = data_type.Deserialize(output_bytes, &s.Parameters)
		if err != nil {
			return nil, fmt.Errorf("data_type.Deserialize %s: %w", string(output_bytes), err)
		}

		logs[i] = s
	}
	return logs, err
}
