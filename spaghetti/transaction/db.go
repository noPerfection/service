package transaction

import (
	"fmt"

	"github.com/blocklords/gosds/db"
)

func DbSave(db *db.Database, t *Transaction) error {
	_, err := db.Connection.Exec(`INSERT IGNORE INTO spaghetti_transactions (network_id, block_number, transaction_id, transaction_from, transaction_to, transaction_index, data, value) VALUES (?, ?, ?, ?, ?, ?, ?, ?) `,
		t.NetworkId, t.BlockNumber, t.Txid, t.TxFrom, t.TxTo, t.TxIndex, t.Data, t.Value)
	if err != nil {
		return err
	}
	return nil
}

// Clears the logs in database for a network id
// Up until the latest_block_number
func DbClear(db *db.Database, network_id string, latest_block_number uint64) error {
	_, err := db.Connection.Exec(`
		DELETE FROM 
			spaghetti_transactions 
		WHERE 
			network_id = ? AND 
			block_number <= ? `,
		network_id, latest_block_number)

	return err
}

func GetForBlock(db *db.Database, network_id string, block_number uint64) ([]*Transaction, error) {
	rows, err := db.Connection.Query(`
		SELECT 
			t.network_id, 
			t.block_number, 
			b.block_timestamp, 
			t.transaction_id, 
			t.transaction_from, 
			t.transaction_to, 
			t.transaction_index, 
			t.data, 
			t.value 
		FROM 
			spaghetti_transactions AS t,
			spaghetti_blocks AS b 
		WHERE 
			t.network_id = ? AND 
			t.block_number = ? AND
			b.network_id = t.network_id AND
			b.block_number = t.block_number `,
		network_id, block_number)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// An txum slice to hold data from returned rows.
	var transactions []*Transaction

	// Loop through rows, using Scan to assign column data to struct fields.
	for rows.Next() {
		var tx Transaction
		if err := rows.Scan(&tx.NetworkId, &tx.BlockNumber, &tx.BlockTimestamp, &tx.Txid,
			&tx.TxFrom, &tx.TxTo, &tx.TxIndex, &tx.Data, &tx.Value); err != nil {
			return transactions, err
		}
		transactions = append(transactions, &tx)
	}
	if err = rows.Err(); err != nil {
		return transactions, err
	}
	rows.Close()
	return transactions, nil
}

func GetForBlockAndTxTo(db *db.Database, network_id string, block_number uint64, to string) ([]*Transaction, error) {
	rows, err := db.Connection.Query(`
		SELECT 
			t.network_id, 
			t.block_number,
			b.block_timestamp, 
			t.transaction_id, 
			t.transaction_from, 
			t.transaction_to, 
			t.transaction_index, 
			t.data, 
			t.value 
		FROM 
			spaghetti_transactions AS t,
			spaghetti_blocsk AS b 
		WHERE 
			t.network_id = ? AND 
			t.block_number = ? AND 
			t.transaction_to = ? AND 
			b.network_id = t.network_id AND 
			b.block_number = t.block_number `, network_id, block_number, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// An txum slice to hold data from returned rows.
	var transactions []*Transaction

	// Loop through rows, using Scan to assign column data to struct fields.
	for rows.Next() {
		var tx Transaction
		if err := rows.Scan(&tx.NetworkId, &tx.BlockNumber, &tx.BlockTimestamp, &tx.Txid,
			&tx.TxFrom, &tx.TxTo, &tx.TxIndex, &tx.Data, &tx.Value); err != nil {
			fmt.Println("Error returned")
			return transactions, err
		}
		transactions = append(transactions, &tx)
	}
	if err = rows.Err(); err != nil {
		return transactions, err
	}
	return transactions, nil
}

func GetForBlockRangeAndTxTo(db *db.Database, network_id string, blockNumberFrom uint64, blockNumberTo uint64, to string) ([]*Transaction, error) {
	rows, err := db.Connection.Query(
		`SELECT 
			t.network_id, 
			t.block_number, 
			b.block_timestamp, 
			t.transaction_id, 
			t.transaction_from, 
			t.transaction_to, 
			t.transaction_index, 
			t.data, 
			t.value 
		FROM 
			spaghetti_transactions AS t,
			spaghetti_blocks AS b 
		WHERE 
			t.network_id = ? AND 
			t.block_number > ? AND 
			t.block_number <= ? AND 
			t.transaction_to = ? AND
			b.network_id = t.network_id AND
			b.block_number = t.block_number `,
		network_id, blockNumberFrom, blockNumberTo, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// An txum slice to hold data from returned rows.
	var transactions []*Transaction

	// Loop through rows, using Scan to assign column data to struct fields.
	for rows.Next() {
		var tx Transaction
		if err := rows.Scan(&tx.NetworkId, &tx.BlockNumber, &tx.BlockTimestamp, &tx.Txid,
			&tx.TxFrom, &tx.TxTo, &tx.TxIndex, &tx.Data, &tx.Value); err != nil {
			fmt.Println("Error returned")
			return transactions, err
		}
		transactions = append(transactions, &tx)
	}
	if err = rows.Err(); err != nil {
		return transactions, err
	}
	return transactions, nil
}

func GetForBlockRangeAndTx(db *db.Database, network_id string, blockNumberFrom uint64, blockNumberTo uint64) ([]*Transaction, error) {
	rows, err := db.Connection.Query(
		`SELECT 
			t.network_id, 
			t.block_number, 
			b.block_timestamp, 
			t.transaction_id, 
			t.transaction_from, 
			t.transaction_to, 
			t.transaction_index, 
			t.data, 
			t.value 
		FROM 
			spaghetti_transactions AS t,
			spaghetti_blocks AS b 
		WHERE 
			t.network_id = ? AND 
			t.block_number > ? AND 
			t.block_number <= ? AND 
			b.network_id = t.network_id AND
			b.block_number = t.block_number `,
		network_id, blockNumberFrom, blockNumberTo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// An txum slice to hold data from returned rows.
	var transactions []*Transaction

	// Loop through rows, using Scan to assign column data to struct fields.
	for rows.Next() {
		var tx Transaction
		if err := rows.Scan(&tx.NetworkId, &tx.BlockNumber, &tx.BlockTimestamp, &tx.Txid,
			&tx.TxFrom, &tx.TxTo, &tx.TxIndex, &tx.Data, &tx.Value); err != nil {
			fmt.Println("Error returned")
			return transactions, err
		}
		transactions = append(transactions, &tx)
	}
	if err = rows.Err(); err != nil {
		return transactions, err
	}
	return transactions, nil
}
