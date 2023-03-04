package event

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/blocklords/gosds/db"
)

func Save(db *db.Database, t *Log) error {
	byt, err := json.Marshal(t.Output)
	if err != nil {
		return fmt.Errorf("json.serialize event outputs %v: %w", t.Output, err)
	}

	_, err = db.Connection.Exec(`INSERT IGNORE INTO categorizer_logs 
	(address, transaction_id, transaction_index, network_id, block_number, block_timestamp, log_index, log, output)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, t.Address, t.TransactionId, t.TransactionIndex, t.NetworkId, t.BlockNumber, t.BlockTimestamp, t.LogIndex, t.Log, byt)

	if err != nil {
		return fmt.Errorf("database exec: %w", err)
	}

	return nil
}

// returns list of logs by transaction keys
func GetLogsFromDb(con *db.Database, txKeys []string) ([]*Log, error) {
	var logs []*Log = make([]*Log, 0)

	return nil, fmt.Errorf("todo: change the transaction keys to the list of network_ids and transaction_ids")

	if len(txKeys) == 0 {
		return logs, nil
	}

	query := `
	SELECT
		logs.block_number, 
		logs.block_timestamp,
		logs.transaction_key,
		logs.log_index,
		logs.address,
		logs.log,
		logs.output
	FROM 
		categorizer_logs AS logs, categorizer_transactions AS txs
	WHERE 
		logs.transaction_key IN (?` + strings.Repeat(",?", len(txKeys)-1) + `) AND txs.transaction_key = logs.transaction_key`

	args := make([]interface{}, len(txKeys))
	for i, param := range txKeys {
		args[i] = param
	}

	rows, err := con.Connection.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("database query: %w", err)
	}
	defer rows.Close()

	// Loop through rows, using Scan to assign column data to struct fields.
	for rows.Next() {
		var s Log
		var outputBytes []byte
		var transaction_key string
		if err := rows.Scan(s.BlockNumber, s.BlockTimestamp, &transaction_key, &s.LogIndex, &s.Address, &s.Log, &outputBytes); err != nil {
			return nil, fmt.Errorf("database row scan: %w", err)
		}

		jsonErr := json.Unmarshal(outputBytes, &s.Output)
		if jsonErr != nil {
			return nil, fmt.Errorf("json.deserialize %s: %w", string(outputBytes), err)
		}

		parts := strings.Split(transaction_key, ".")
		s.NetworkId = parts[0]
		s.TransactionId = parts[1]

		logs = append(logs, &s)
	}

	return logs, nil
}
