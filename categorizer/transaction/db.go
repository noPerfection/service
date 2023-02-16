package transaction

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/blocklords/gosds/db"
)

func Save(db *db.Database, t *Transaction) error {
	byt, err := json.Marshal(t.Args)
	if err != nil {
		return err
	}

	key := t.NetworkId + "." + t.Address

	_, err = db.Connection.Exec(`INSERT IGNORE INTO categorizer_transactions 
	(transaction_key, network_id, address, block_number, block_timestamp, transaction_id, transaction_index, transaction_from, method_name, args, value)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, key, t.NetworkId, t.Address, t.BlockNumber, t.BlockTimestamp, t.Txid, t.TxIndex, t.TxFrom, t.Method, byt, t.Value)
	return err
}

// blockFrom and blockTo are timestamps
func TransactionAmount(db *db.Database, blockFrom uint64, blockTo uint64, keys []string) (uint, error) {
	if len(keys) == 0 {
		return 0, nil
	}

	query := `
	SELECT 
		COUNT(txId) as txAmount 
	FROM 
		categorizer_transactions 
	WHERE 
    	block_timestamp > ? AND 
		block_timestamp <= ? AND 
		id IN (?` + strings.Repeat(",?", len(keys)-1) +
		`)`

	firstParams := 2
	args := make([]interface{}, len(keys)+firstParams)
	args[0] = blockFrom
	args[1] = blockTo
	for i, key := range keys {
		args[i+firstParams] = key
	}

	var txAmount uint
	err := db.Connection.QueryRow(query, args...).Scan(&txAmount)
	if err != nil {
		fmt.Println("Static Abi loading abi returned db error: ", err.Error())
		return 0, err
	}

	return txAmount, nil
}

// blockFrom and blockTo are timestamps
func TransactionGetAll(con *db.Database, blockFrom uint64, blockTo uint64, keys []string, page uint64, limit uint64) ([]*Transaction, error) {
	if len(keys) == 0 {
		return []*Transaction{}, nil
	}

	query := `
	SELECT
		network_id,
		address,
		block_number,
		block_timestamp,
		transaction_id,
		transaction_index,
		transaction_from,
		method_name,
		args,
		value
	FROM 
		categorizer_transactions 
	WHERE  
    	block_timestamp > ? AND 
		block_timestamp <= ? AND 
		transaction_key IN (?` + strings.Repeat(",?", len(keys)-1) + `)
	ORDER BY 
		block_timestamp ASC
	LIMIT ? OFFSET ? `

	firstParams := 2
	args := make([]interface{}, len(keys)+firstParams+2)
	args[0] = blockFrom
	args[1] = blockTo
	for i, key := range keys {
		args[i+firstParams] = key
	}
	arg_offset := len(keys) + firstParams
	args[arg_offset] = limit
	args[arg_offset+1] = (page - 1) * limit

	rows, err := con.Connection.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txs []*Transaction

	// Loop through rows, using Scan to assign column data to struct fields.
	for rows.Next() {
		var s Transaction
		var argBytes []byte
		if err := rows.Scan(&s.NetworkId, &s.Address, &s.BlockNumber, &s.BlockTimestamp, &s.Txid, &s.TxIndex, &s.TxFrom, &s.Method, &argBytes, &s.Value); err != nil {
			return nil, err
		}

		jsonErr := json.Unmarshal(argBytes, &s.Args)
		if jsonErr != nil {
			return nil, jsonErr
		}

		txs = append(txs, &s)
	}
	rows.Close()

	return txs, nil
}

// Returns the most recent timestamp
func GetRecentBlockTimestamp(db *db.Database, keys []string) (uint64, error) {
	if len(keys) == 0 {
		return 0, nil
	}

	args := make([]interface{}, len(keys))
	for i, key := range keys {
		args[i] = key
	}

	query := `
	SELECT
		block_timestamp
	FROM 
		categorizer_transactions 
	WHERE  
		transaction_key IN (?` + strings.Repeat(",?", len(keys)-1) + `)
	ORDER BY 
		block_timestamp DESC
	LIMIT 1 `

	var block_timestamp uint64
	err := db.Connection.QueryRow(query, args...).Scan(&block_timestamp)
	if err != nil {
		return 0, err
	}

	return block_timestamp, nil
}
