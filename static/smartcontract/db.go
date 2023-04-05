package smartcontract

import (
	"fmt"

	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/smartcontract_key"
	"github.com/blocklords/sds/db"
)

// Whether the smartcontract address on network_id exist in database or not
func ExistInDatabase(db *db.Database, key smartcontract_key.Key) bool {
	var exists bool
	err := db.Connection.QueryRow("SELECT IF(COUNT(address),'true','false') FROM static_smartcontract WHERE network_id = ? AND address = ?", key.NetworkId, key.Address).Scan(&exists)
	if err != nil {
		fmt.Println("Static Smartcontract exists returned db error: ", err.Error())
		return false
	}

	return exists
}

func SetInDatabase(db *db.Database, a *Smartcontract) error {
	result, err := db.Connection.Exec(`
		INSERT IGNORE INTO 
			static_smartcontract (
				network_id, 
				address, 
				abi_id, 
				transaction_id, 
				transaction_index,
				block_number, 
				block_timestamp, 
				deployer
			) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?) `,
		a.SmartcontractKey.NetworkId,
		a.SmartcontractKey.Address,
		a.AbiId,
		a.TransactionKey.Id,
		a.TransactionKey.Index,
		a.BlockHeader.Number,
		a.BlockHeader.Timestamp,
		a.Deployer,
	)
	if err != nil {
		return fmt.Errorf("db.Insert network id = %s, address = %s: %w", a.SmartcontractKey.NetworkId, a.SmartcontractKey.Address, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking insert result: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("expected to have 1 affected rows. Got %d", affected)
	}

	return nil
}

// Returns the smartcontract by address on network_id from database
func GetFromDatabase(db *db.Database, key smartcontract_key.Key) (*Smartcontract, error) {
	query := `SELECT abi_id, transaction_id, transaction_index, block_number, block_timestamp, deployer FROM static_smartcontract WHERE network_id = ? AND address = ?`

	var s Smartcontract = Smartcontract{
		SmartcontractKey: key,
		TransactionKey:   blockchain.TransactionKey{},
		BlockHeader:      blockchain.BlockHeader{},
	}

	row := db.Connection.QueryRow(query, key.NetworkId, key.Address)
	if err := row.Scan(&s.AbiId, &s.TransactionKey.Id, &s.TransactionKey.Index, &s.BlockHeader.Number, &s.BlockHeader.Timestamp, &s.Deployer); err != nil {
		return nil, err
	}

	return &s, nil
}
