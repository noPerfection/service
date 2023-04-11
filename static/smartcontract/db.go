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
		Fields: []string{"network_id",
			"address",
			"abi_id",
			"transaction_id",
			"transaction_index",
			"block_number",
			"block_timestamp",
			"deployer"},
		Tables: []string{"static_smartcontract"},
		Arguments: []interface{}{
			a.SmartcontractKey.NetworkId,
			a.SmartcontractKey.Address,
			a.AbiId,
			a.TransactionKey.Id,
			a.TransactionKey.Index,
			a.BlockHeader.Number,
			a.BlockHeader.Timestamp,
			a.Deployer,
		},
	}
	var reply handler.InsertReply

	err := handler.INSERT.Request(db, request, &reply)
	if err != nil {
		return fmt.Errorf("handler.WRITE.Push: %w", err)
	}
	return nil
}

func GetAllFromDatabase(db *remote.ClientSocket) ([]*Smartcontract, error) {
	request := handler.DatabaseQueryRequest{
		Fields: []string{
			"network_id",
			"address",
			"abi_id",
			"transaction_id",
			"transaction_index",
			"block_number",
			"block_timestamp",
			"deployer",
		},
		Tables: []string{"static_smartcontract"},
	}
	var reply handler.SelectAllReply

	err := handler.SELECT_ALL.Request(db, request, &reply)
	if err != nil {
		return nil, fmt.Errorf("handler.SELECT_ALL.Request: %w", err)
	}

	smartcontracts := make([]*Smartcontract, len(reply.Rows))

	// Loop through rows, using Scan to assign column data to struct fields.
	for i, raw := range reply.Rows {
		var sm = Smartcontract{
			SmartcontractKey: smartcontract_key.Key{},
			TransactionKey:   blockchain.TransactionKey{},
			BlockHeader:      blockchain.BlockHeader{},
		}

		err := raw.ToInterface(&sm.SmartcontractKey)
		if err != nil {
			return nil, fmt.Errorf("failed to extract smartcontract key from database result: %w", err)
		}

		err = raw.ToInterface(&sm.BlockHeader)
		if err != nil {
			return nil, fmt.Errorf("failed to extract smartcontract key from database result: %w", err)
		}

		err = raw.ToInterface(&sm.TransactionKey)
		if err != nil {
			return nil, fmt.Errorf("failed to extract smartcontract key from database result: %w", err)
		}

		deployer, err := raw.GetString("deployer")
		if err != nil {
			return nil, fmt.Errorf("failed to extract deployer from database result: %w", err)
		}
		sm.Deployer = deployer

		abi_id, err := raw.GetString("abi_id")
		if err != nil {
			return nil, fmt.Errorf("failed to extract abi id from database result: %w", err)
		}
		sm.AbiId = abi_id

		smartcontracts[i] = &sm
	}
	return smartcontracts, err
}
