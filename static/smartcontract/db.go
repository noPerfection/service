package smartcontract

import (
	"fmt"

	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/smartcontract_key"
	"github.com/blocklords/sds/db"
)

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

func GetAllFromDatabase(db *db.Database) ([]*Smartcontract, error) {
	rows, err := db.Connection.Query("SELECT network_id, address, abi_id, transaction_id, transaction_index, block_number, block_timestamp, deployer FROM static_smartcontract WHERE 1")
	if err != nil {
		return nil, fmt.Errorf("db: %w", err)
	}

	defer rows.Close()

	smartcontracts := make([]*Smartcontract, 0)

	// Loop through rows, using Scan to assign column data to struct fields.
	for rows.Next() {
		var s Smartcontract = Smartcontract{
			SmartcontractKey: smartcontract_key.Key{},
			TransactionKey:   blockchain.TransactionKey{},
			BlockHeader:      blockchain.BlockHeader{},
		}

		if err := rows.Scan(
			&s.SmartcontractKey.NetworkId,
			&s.SmartcontractKey.Address,
			&s.AbiId,
			&s.TransactionKey.Id,
			&s.TransactionKey.Index,
			&s.BlockHeader.Number,
			&s.BlockHeader.Timestamp,
			&s.Deployer); err != nil {
			return nil, fmt.Errorf("failed to scan database result: %w", err)
		}

		smartcontracts = append(smartcontracts, &s)
	}
	return smartcontracts, err
}
