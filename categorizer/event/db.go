package event

import (
	"encoding/json"
	"strings"

	"github.com/blocklords/gosds/db"
)

func Save(db *db.Database, t *Log) error {
	byt, err := json.Marshal(t.Output)
	if err != nil {
		return err
	}

	transaction_key := t.NetworkId + "." + t.Txid

	_, err = db.Connection.Exec(`INSERT IGNORE INTO categorizer_logs 
	(address, transaction_key, log_index, log, output)
	VALUES (?, ?, ?, ?, ?)`, t.Address, transaction_key, t.LogIndex, t.Log, byt)
	return err
}

// returns list of logs by transaction keys
func GetLogsFromDb(con *db.Database, txKeys []string) ([]*Log, error) {
	var logs []*Log = make([]*Log, 0)

	if len(txKeys) == 0 {
		return logs, nil
	}

	query := `
	SELECT
		txs.block_number, 
		txs.block_timestamp,
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
		return nil, err
	}
	defer rows.Close()

	// Loop through rows, using Scan to assign column data to struct fields.
	for rows.Next() {
		var s Log
		var outputBytes []byte
		var transaction_key string
		if err := rows.Scan(s.BlockNumber, s.BlockTimestamp, &transaction_key, &s.LogIndex, &s.Address, &s.Log, &outputBytes); err != nil {
			return nil, err
		}

		jsonErr := json.Unmarshal(outputBytes, &s.Output)
		if jsonErr != nil {
			return nil, jsonErr
		}

		parts := strings.Split(transaction_key, ".")
		s.NetworkId = parts[0]
		s.Txid = parts[1]

		logs = append(logs, &s)
	}

	return logs, nil
}
