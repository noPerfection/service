// The db.go contains the database related functions of the ABI
package abi

import (
	"fmt"

	"github.com/blocklords/sds/db"
)

// Save the ABI in the Database
func SetInDatabase(db *db.Database, a *Abi) error {
	_, err := db.Connection.Exec(`INSERT IGNORE INTO static_abi (abi_id, body) VALUES (?, ?) `, a.Id, a.Bytes)
	if err != nil {
		return fmt.Errorf("abi setting db error: %v", err)
	}
	return nil
}

// Returns the Abi from database by its abi_hash
func GetFromDatabaseByAbiHash(db *db.Database, abi_hash string) (*Abi, error) {
	var bytes []byte
	abi := Abi{}
	abi.Id = abi_hash
	err := db.Connection.QueryRow("SELECT body FROM static_abi WHERE abi_id = ? ", abi_hash).Scan(&bytes)
	if err != nil {
		return nil, fmt.Errorf("db: %w", err)
	}

	built, err := NewFromBytes(bytes)
	return built, err
}

// Returns the Abi by the smartcontract key (network id . address)
func GetFromDatabase(db *db.Database, network_id string, address string) (*Abi, error) {
	var bytes []byte
	var abi_hash string

	err := db.Connection.QueryRow(`
		SELECT 
			static_abi.body,
			static_abi.abi_id
		FROM 
			static_abi, static_smartcontract 
		WHERE 
			static_abi.abi_id = static_smartcontract.abi_id AND
			static_smartcontract.network_id = ? AND
			static_smartcontract.address = ?
		`, network_id, address).Scan(&bytes, &abi_hash)
	if err != nil {
		fmt.Println("Static Abi loading abi returned db error: ", err.Error())
		return nil, err
	}

	built, err := NewFromBytes(bytes)
	if err == nil {
		built.Id = abi_hash
	}

	return built, err
}

// Checks whether the Abi exists in the database or not
func ExistInDatabase(db *db.Database, abi_hash string) bool {
	var exists bool
	err := db.Connection.QueryRow("SELECT IF(COUNT(abi_id),'true','false') FROM static_abi WHERE abi_id = ? ", abi_hash).Scan(&exists)
	if err != nil {
		fmt.Println("Static Abi exists returned db error: ", err.Error())
		return false
	}

	return exists
}
