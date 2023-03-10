package smartcontract

import (
	"fmt"

	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/smartcontract_key"
	"github.com/blocklords/sds/db"
)

// Set the block parameters in the database
func SaveBlockParameters(db *db.Database, sm *Smartcontract) error {
	_, err := db.Connection.Exec(`UPDATE categorizer_smartcontract SET block_number = ?, block_timestamp = ? WHERE network_id = ? AND address = ? `,
		sm.Block.Number, sm.Block.Timestamp, sm.Key.NetworkId, sm.Key.Address)
	if err != nil {
		return fmt.Errorf("failed to update the categorized block data in the database %s %s: %w ", sm.Key.NetworkId, sm.Key.Address, err)
	}

	return nil
}

func Exists(db *db.Database, key smartcontract_key.Key) bool {
	var exists bool
	err := db.Connection.QueryRow("SELECT IF(COUNT(address),'true','false') FROM categorizer_smartcontract WHERE network_id = ? AND address = ? ", key.NetworkId, key.Address).Scan(&exists)
	if err != nil {
		fmt.Println("Categorizer checking error: ", err.Error())
		return false
	}

	return exists
}

func Save(db *db.Database, b *Smartcontract) error {
	_, err := db.Connection.Exec(`INSERT IGNORE INTO categorizer_smartcontract (network_id, address, block_number, block_timestamp) VALUES (?, ?, ?, ?) `,
		b.Key.NetworkId, b.Key.Address, b.Block.Number, b.Block.Timestamp)
	if err != nil {
		return fmt.Errorf("failed to set smartcontract in database %s %s: %w ", b.Key.NetworkId, b.Key.Address, err)
	}
	return nil
}

// Return the single smartcontract from database
func Get(db *db.Database, key smartcontract_key.Key) (*Smartcontract, error) {
	row := db.Connection.QueryRow("SELECT block_number, block_timestamp FROM categorizer_smartcontract WHERE network_id = ? AND address = ? ", key.NetworkId, key.Address)

	// Loop through rows, using Scan to assign column data to struct fields.
	var block blockchain.BlockHeader
	if err := row.Scan(&block.Number, &block.Timestamp); err != nil {
		return nil, fmt.Errorf("row.Scan from the database: %w ", err)
	}
	sm := Smartcontract{
		Key:   key,
		Block: block,
	}

	return &sm, nil
}

func GetAll(db *db.Database) ([]*Smartcontract, error) {
	smartcontracts := make([]*Smartcontract, 0)

	rows, err := db.Connection.Query("SELECT network_id, address, block_number, block_timestamp FROM categorizer_smartcontract ")
	if err != nil {
		return smartcontracts, fmt.Errorf("database.Query: %w", err)
	}
	defer rows.Close()

	// Loop through rows, using Scan to assign column data to struct fields.
	for rows.Next() {
		sm := Smartcontract{
			Key:   smartcontract_key.Key{},
			Block: blockchain.BlockHeader{},
		}
		if err := rows.Scan(&sm.Key.NetworkId, &sm.Key.Address, &sm.Block.Number, &sm.Block.Timestamp); err != nil {
			return smartcontracts, fmt.Errorf("row.Scan: %w", err)
		}

		smartcontracts = append(smartcontracts, &sm)
	}
	if err = rows.Err(); err != nil {
		return smartcontracts, fmt.Errorf("database error: %w", err)
	}

	return smartcontracts, nil
}

// Returns list of categorizing smartcontracts at certain network.
func GetAllByNetworkId(db *db.Database, network_id string) ([]*Smartcontract, error) {
	var smartcontracts []*Smartcontract

	rows, err := db.Connection.Query("SELECT network_id, address, block_number, block_timestamp FROM categorizer_smartcontract WHERE network_id = ?", network_id)
	if err != nil {
		return nil, fmt.Errorf("failed to query all categorizer smartcontracts for %s network_id: %w", network_id, err)
	}
	defer rows.Close()
	// An album slice to hold data from returned rows.

	// Loop through rows, using Scan to assign column data to struct fields.
	for rows.Next() {
		sm := Smartcontract{
			Key:   smartcontract_key.Key{},
			Block: blockchain.BlockHeader{},
		}

		if err := rows.Scan(&sm.Key.NetworkId, &sm.Key.Address, &sm.Block.Number, &sm.Block.Timestamp); err != nil {
			return nil, fmt.Errorf("row.Scan: %w", err)
		}

		smartcontracts = append(smartcontracts, &sm)
	}
	if err = rows.Err(); err != nil {
		return smartcontracts, fmt.Errorf("database error: %w", err)
	}

	return smartcontracts, nil
}
