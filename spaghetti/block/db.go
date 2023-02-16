package block

import (
	"database/sql"

	"github.com/blocklords/gosds/db"
)

// Returns the recent block number in the database for a network_id
func GetRecentBlockNumber(db *db.Database, network_id string) (uint64, error) {
	row := db.Connection.QueryRow("SELECT block_number FROM spaghetti_blocks WHERE network_id = ? ORDER BY block_number DESC LIMIT 1 ", network_id)

	var block_number uint64 = 0
	err := row.Scan(&block_number)

	return block_number, err
}

// Returns the earliest block number in the database for a network_id
func GetEarliestBlockNumber(db *db.Database, network_id string) (uint64, error) {
	row := db.Connection.QueryRow("SELECT block_number FROM spaghetti_blocks WHERE network_id = ? ORDER BY block_number ASC LIMIT 1 ", network_id)

	var block_number uint64 = 0
	err := row.Scan(&block_number)

	return block_number, err
}

// Returns the most recent block number that is less than the passed block timestamp
func GetLatestBlockNumber(db *db.Database, network_id string, block_timestamp uint64) (uint64, error) {
	row := db.Connection.QueryRow("SELECT block_number FROM spaghetti_blocks WHERE network_id = ? AND block_timestamp < ? ORDER BY block_timestamp DESC LIMIT 1 ", network_id, block_timestamp)

	var block_number uint64 = 0
	err := row.Scan(&block_number)
	if err != nil && err == sql.ErrNoRows {
		return 0, nil
	}

	return block_timestamp, err
}

// Returns the block timestamp from the database
func GetBlockTimestamp(db *db.Database, network_id string, block_number uint64) (uint64, error) {
	row := db.Connection.QueryRow("SELECT block_timestamp FROM spaghetti_blocks WHERE network_id = ? AND block_number = ? ", network_id, block_number)

	var block_timestamp uint64 = 0
	err := row.Scan(&block_timestamp)

	return block_timestamp, err
}

// Sets the block information, on success returns the SpaghettiBlock
func SetBlock(db *db.Database, network_id string, block_number uint64, transaction_amount uint, log_amount uint, block_timestamp uint64) error {
	_, err := db.Connection.Exec(`INSERT IGNORE INTO spaghetti_blocks (network_id, block_number, transaction_amount, log_amount, block_timestamp)
	VALUES (?, ?, ?, ?, ?)`, network_id, block_number, transaction_amount, log_amount, block_timestamp)
	return err
}

// Clears the logs in database for a network id
// Up until the latest_block_number
func Clear(db *db.Database, network_id string, latest_block_number uint64) error {
	_, err := db.Connection.Exec(`
		DELETE FROM 
			spaghetti_blocks
		WHERE 
			network_id = ? AND 
			block_number <= ? `,
		network_id, latest_block_number)

	return err
}
