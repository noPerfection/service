// The db.go contains the database related functions of the ABI
package abi

import (
	"fmt"

	"github.com/blocklords/gosds/db"
)

// Save the ABI in the Database
func SetInDatabase(db *db.Database, a *Abi) error {
	_, err := db.Connection.Exec(`INSERT IGNORE INTO static_abi (abi_hash, abi) VALUES (?, ?) `, a.AbiHash, a.Bytes)
	if err != nil {
		return fmt.Errorf("abi setting db error: %v", err)
	}
	a.SetExists(true)
	return nil
}

// Returns the Abi from database by its abi_hash
func GetFromDatabaseByAbiHash(db *db.Database, abi_hash string) *Abi {
	var bytes []byte
	abi := Abi{}
	abi.AbiHash = abi_hash
	abi.SetExists(false)
	err := db.Connection.QueryRow("SELECT abi FROM static_abi WHERE abi_hash = ? ", abi_hash).Scan(&bytes)
	if err != nil {
		fmt.Println("Static Abi loading abi returned db error: ", err.Error())
		return &abi
	}

	built := FromBytes(bytes)
	built.SetExists(true)

	return built
}

// Returns the Abi hash by the smartcontract key (network id . address)
func GetFromDatabase(db *db.Database, network_id string, address string) (*Abi, error) {
	var bytes []byte
	var abi_hash string

	abi := Abi{}
	abi.SetExists(false)
	err := db.Connection.QueryRow(`
		SELECT 
			static_abi.abi,
			static_abi.abi_hash
		FROM 
			static_abi, static_smartcontract 
		WHERE 
			static_abi.abi_hash = static_smartcontract.abi_hash AND
			static_smartcontract.network_id = ? AND
			static_smartcontract.address = ?
		`, network_id, address).Scan(&bytes, &abi_hash)
	if err != nil {
		fmt.Println("Static Abi loading abi returned db error: ", err.Error())
		return nil, err
	}

	built := FromBytes(bytes)
	built.SetExists(true)
	built.AbiHash = abi_hash

	return built, nil
}

// Checks whether the Abi exists in the database or not
func ExistInDatabase(db *db.Database, abi_hash string) bool {
	var exists bool
	err := db.Connection.QueryRow("SELECT IF(COUNT(abi_hash),'true','false') FROM static_abi WHERE abi_hash = ? ", abi_hash).Scan(&exists)
	if err != nil {
		fmt.Println("Static Abi exists returned db error: ", err.Error())
		return false
	}

	return exists
}
