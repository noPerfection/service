package smartcontract

import (
	"fmt"

	"github.com/blocklords/sds/db"
)

// Update the block number and block timestamp of the smartcontract
func SetSyncing(db *db.Database, b *Smartcontract, n uint64, t uint64) error {
	b.SetBlockParameter(n, t)
	_, err := db.Connection.Exec(`UPDATE categorizer_smartcontracts SET categorized_block_number = ?, categorized_block_timestamp = ? WHERE network_id = ? AND address = ? `,
		n, t, b.NetworkId, b.Address)
	if err != nil {
		return fmt.Errorf("failed to update the categorized block data in the database %s %s: %w ", b.NetworkId, b.Address, err)
	}

	return nil
}

func Exists(db *db.Database, network_id string, address string) bool {
	var exists bool
	err := db.Connection.QueryRow("SELECT IF(COUNT(address),'true','false') FROM categorizer_smartcontracts WHERE network_id = ? AND address = ? ", network_id, address).Scan(&exists)
	if err != nil {
		fmt.Println("Categorizer checking error: ", err.Error())
		return false
	}

	return exists
}

func Save(db *db.Database, b *Smartcontract) error {
	_, err := db.Connection.Exec(`INSERT IGNORE INTO categorizer_smartcontracts (network_id, address, categorized_block_number, categorized_block_timestamp) VALUES (?, ?, ?, ?) `,
		b.NetworkId, b.Address, b.CategorizedBlockNumber, b.CategorizedBlockTimestamp)
	if err != nil {
		return fmt.Errorf("failed to set smartcontract in database %s %s: %w ", b.NetworkId, b.Address, err)
	}
	return nil
}

// Return the single smartcontract from database
func Get(db *db.Database, network_id string, address string) (*Smartcontract, error) {
	row := db.Connection.QueryRow("SELECT categorized_block_number, categorized_block_timestamp FROM categorizer_smartcontracts WHERE network_id = ? AND address = ? ", network_id, address)

	// Loop through rows, using Scan to assign column data to struct fields.
	var categorized_block_number uint64
	var categorized_block_timestamp uint64
	if err := row.Scan(&categorized_block_number, &categorized_block_timestamp); err != nil {
		return nil, fmt.Errorf("row.Scan from the database: %w ", err)
	}
	sm := Smartcontract{
		NetworkId:                 network_id,
		Address:                   address,
		CategorizedBlockNumber:    categorized_block_number,
		CategorizedBlockTimestamp: categorized_block_timestamp,
	}

	return &sm, nil
}

func GetAll(db *db.Database) ([]*Smartcontract, error) {
	smartcontracts := make([]*Smartcontract, 0)

	rows, err := db.Connection.Query("SELECT network_id, address, categorized_block_number, categorized_block_timestamp FROM categorizer_smartcontracts ")
	if err != nil {
		return smartcontracts, fmt.Errorf("database.Query: %w", err)
	}
	defer rows.Close()

	// Loop through rows, using Scan to assign column data to struct fields.
	for rows.Next() {
		var network_id string
		var address string
		var categorized_block_number uint64
		var categorized_block_timestamp uint64
		if err := rows.Scan(&network_id, &address, &categorized_block_number, &categorized_block_timestamp); err != nil {
			return smartcontracts, fmt.Errorf("row.Scan: %w", err)
		}
		sm := Smartcontract{
			NetworkId:                 network_id,
			Address:                   address,
			CategorizedBlockNumber:    categorized_block_number,
			CategorizedBlockTimestamp: categorized_block_timestamp,
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

	rows, err := db.Connection.Query("SELECT network_id, address, categorized_block_number, categorized_block_timestamp FROM categorizer_smartcontracts WHERE network_id = ?", network_id)
	if err != nil {
		return nil, fmt.Errorf("failed to query all categorizer smartcontracts for %s network_id: %w", network_id, err)
	}
	defer rows.Close()
	// An album slice to hold data from returned rows.

	// Loop through rows, using Scan to assign column data to struct fields.
	for rows.Next() {
		var network_id string
		var address string
		var categorized_block_number uint64
		var categorized_block_timestamp uint64
		if err := rows.Scan(&network_id, &address, &categorized_block_number, &categorized_block_timestamp); err != nil {
			return nil, fmt.Errorf("row.Scan: %w", err)
		}
		sm := Smartcontract{
			NetworkId:                 network_id,
			Address:                   address,
			CategorizedBlockNumber:    categorized_block_number,
			CategorizedBlockTimestamp: categorized_block_timestamp,
		}

		smartcontracts = append(smartcontracts, &sm)
	}
	if err = rows.Err(); err != nil {
		return smartcontracts, fmt.Errorf("database error: %w", err)
	}

	return smartcontracts, nil
}
