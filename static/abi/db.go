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

// Returns the Abi from database by its abi_id
func GetFromDatabaseByAbiId(db *db.Database, abi_id string) (*Abi, error) {
	var bytes []byte
	abi := Abi{}
	abi.Id = abi_id
	err := db.Connection.QueryRow("SELECT body FROM static_abi WHERE abi_id = ? ", abi_id).Scan(&bytes)
	if err != nil {
		return nil, fmt.Errorf("db: %w", err)
	}

	built, err := NewFromBytes(bytes)
	return built, err
}

// Checks whether the Abi exists in the database or not
func ExistInDatabase(db *db.Database, abi_id string) bool {
	var exists bool
	err := db.Connection.QueryRow("SELECT IF(COUNT(abi_id),'true','false') FROM static_abi WHERE abi_id = ? ", abi_id).Scan(&exists)
	if err != nil {
		fmt.Println("Static Abi exists returned db error: ", err.Error())
		return false
	}

	return exists
}
