package log

import (
	"fmt"

	"github.com/blocklords/gosds/db"
)

func GetForBlock(db *db.Database, network_id string, block_number uint64) ([]*Log, error) {
	rows, err := db.Connection.Query(`
		SELECT 
			l.block_number,
			b.block_timestamp,
			l.network_id, 
			l.transaction_id, 
			l.log_index, 
			l.address,
	        l.data, 
			l.topics 
		FROM 
			spaghetti_logs AS l, spaghetti_blocks AS b
		WHERE 
			l.network_id = ? AND 
			l.block_number = ? AND 
			b.network_id = l.network_id AND
			b.block_number = l.block_number `,
		network_id,
		block_number,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*Log

	// Loop through rows, using Scan to assign column data to struct fields.
	for rows.Next() {
		var l Log
		var topicRaw []byte
		var address interface{}

		if err := rows.Scan(&l.BlockNumber, &l.BlockTimestamp, &l.NetworkId, &l.Txid,
			&l.LogIndex, &address, &l.Data, &topicRaw); err != nil {
			fmt.Println("Error returned ", err)
			return logs, err
		}
		if err := l.ParseTopics(topicRaw); err != nil {
			fmt.Println("Error returned to parse topics ", err)
			return logs, err
		}
		if address != nil {
			l.Address = address.(string)
		}
		logs = append(logs, &l)
	}
	if err = rows.Err(); err != nil {
		return logs, err
	}
	return logs, nil
}

func GetForBlockAndTxTo(db *db.Database, network_id string, blockNumberFrom uint64, blockNumberTo uint64, to string) ([]*Log, error) {
	rows, err := db.Connection.Query(`
		SELECT 
			l.block_number,
			b.block_timestamp,
			l.network_id, 
			l.transaction_id, 
			l.log_index,
			l.address, 
			l.data, 
			l.topics 
		FROM 
			spaghetti_logs AS l, 
			spaghetti_blocks AS b
		WHERE 
			l.network_id = ? AND 
			l.block_number > ? AND 
			l.block_number <= ? AND 
			l.address = ? AND 
			b.network_id = l.network_id AND
			b.block_number = l.block_number `,
		network_id, blockNumberFrom, blockNumberTo, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*Log

	// Loop through rows, using Scan to assign column data to struct fields.
	for rows.Next() {
		var l Log
		var topicRaw []byte
		var address interface{}

		if err := rows.Scan(&l.BlockNumber, &l.BlockTimestamp, &l.NetworkId, &l.Txid,
			&l.LogIndex, &address, &l.Data, &topicRaw); err != nil {
			fmt.Println("Error returned ", err)
			return logs, err
		}
		if err := l.ParseTopics(topicRaw); err != nil {
			fmt.Println("Error returned to parse topics ", err)
			return logs, err
		}
		if address != nil {
			l.Address = address.(string)
		}
		logs = append(logs, &l)
	}
	if err = rows.Err(); err != nil {
		return logs, err
	}
	return logs, nil
}

func GetForBlockAndTx(db *db.Database, network_id string, blockNumberFrom uint64, blockNumberTo uint64) ([]*Log, error) {
	rows, err := db.Connection.Query(`
		SELECT 
			l.block_number,
			b.block_timestamp,
			l.network_id, 
			l.transaction_id, 
			l.log_index,
			l.address, 
			l.data, 
			l.topics 
		FROM 
			spaghetti_logs AS l, 
			spaghetti_blocks AS b
		WHERE 
			l.network_id = ? AND 
			l.block_number > ? AND 
			l.block_number <= ? AND 
			b.network_id = l.network_id AND
			b.block_number = l.block_number `,
		network_id, blockNumberFrom, blockNumberTo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*Log

	// Loop through rows, using Scan to assign column data to struct fields.
	for rows.Next() {
		var l Log
		var topicRaw []byte
		var address interface{}

		if err := rows.Scan(&l.BlockNumber, &l.BlockTimestamp, &l.NetworkId, &l.Txid,
			&l.LogIndex, &address, &l.Data, &topicRaw); err != nil {
			fmt.Println("Error returned ", err)
			return logs, err
		}
		if err := l.ParseTopics(topicRaw); err != nil {
			fmt.Println("Error returned to parse topics ", err)
			return logs, err
		}
		if address != nil {
			l.Address = address.(string)
		}
		logs = append(logs, &l)
	}
	if err = rows.Err(); err != nil {
		return logs, err
	}
	return logs, nil
}

func DbSave(db *db.Database, t *Log) error {
	_, err := db.Connection.Exec(`INSERT IGNORE INTO spaghetti_logs (network_id, transaction_id, address, log_index, data, topics) VALUES (?, ?, ?, ?, ?, ?) `,
		t.NetworkId, t.Txid, t.Address, t.LogIndex, t.Data, t.TopicRaw())
	return err
}

// Clears the logs in database for a network id
// Up until the latest_block_number
func DbClear(db *db.Database, network_id string, latest_block_number uint64) error {
	_, err := db.Connection.Exec(`
		DELETE FROM 
			spaghetti_logs 
		WHERE 
			network_id = ? AND 
			block_number <= ? `,
		network_id, latest_block_number)

	return err
}
