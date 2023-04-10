package smartcontract

import (
	"fmt"

	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/smartcontract_key"
	"github.com/blocklords/sds/db/handler"
)

func SetInDatabase(db *remote.ClientSocket, a *Smartcontract) error {
	request := handler.DatabaseQueryRequest{
		Query: `
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
		Arguments: []interface{}{a.SmartcontractKey.NetworkId,
			a.SmartcontractKey.Address,
			a.AbiId,
			a.TransactionKey.Id,
			a.TransactionKey.Index,
			a.BlockHeader.Number,
			a.BlockHeader.Timestamp,
			a.Deployer},
	}
	var reply handler.WriteReply

	err := handler.WRITE.Request(db, request, &reply)
	if err != nil {
		return fmt.Errorf("handler.WRITE.Push: %w", err)
	}
	return nil
}

func GetAllFromDatabase(db *remote.ClientSocket) ([]*Smartcontract, error) {
	request := handler.DatabaseQueryRequest{
		Query:     "SELECT network_id, address, abi_id, transaction_id, transaction_index, block_number, block_timestamp, deployer FROM static_smartcontract",
		Arguments: []interface{}{},
		Outputs:   []interface{}{"", "", "", "", uint64(0), uint64(0), uint64(0), ""},
	}
	var reply handler.ReadAllReply

	err := handler.WRITE.Request(db, request, &reply)
	if err != nil {
		return nil, fmt.Errorf("handler.WRITE.Push: %w", err)
	}

	smartcontracts := make([]*Smartcontract, len(reply.Rows))

	// Loop through rows, using Scan to assign column data to struct fields.
	for i, raw := range reply.Rows {
		var sm = Smartcontract{
			SmartcontractKey: smartcontract_key.Key{},
			TransactionKey:   blockchain.TransactionKey{},
			BlockHeader:      blockchain.BlockHeader{},
		}

		sm.SmartcontractKey.NetworkId = raw.Outputs[0].(string)
		sm.SmartcontractKey.Address = raw.Outputs[1].(string)
		sm.TransactionKey.Id = raw.Outputs[2].(string)
		sm.TransactionKey.Index = uint(raw.Outputs[3].(uint64))
		sm.BlockHeader.Number = blockchain.Number(raw.Outputs[4].(uint64))
		sm.BlockHeader.Timestamp = blockchain.Timestamp(raw.Outputs[5].(uint64))
		sm.Deployer = raw.Outputs[6].(string)

		smartcontracts[i] = &sm
	}
	return smartcontracts, err
}
