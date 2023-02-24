package handler

import (
	"github.com/blocklords/gosds/categorizer/log"
	"github.com/blocklords/gosds/db"

	"github.com/blocklords/gosds/app/remote/message"
	"github.com/blocklords/gosds/common/data_type"
	"github.com/blocklords/gosds/common/data_type/key_value"
)

// Get's the list of transactions and logs for a particular smartcontract
// Within the range of the timestamp
//
// Request parameters:
// 1. "block_timestamp_from"
// 2. "block_timestamp_to"
// 3. "smartcontract_key"
// 4. "page"
// 5. "limit"
//
// Reply parameters:
// 1. transactions
// 2. logs
// 3. network_id
// 4. address
// 5. block_timestamp
func GetSnapshot(db *db.Database, request message.Request) message.Reply {
	/////////////////////////////////////////////////////////////////////////////
	//
	// Extract the parameters
	//
	/////////////////////////////////////////////////////////////////////////////
	// block_timestamp_from, err := request.Parameters.GetUint64("block_timestamp_from")
	// if err != nil {
	// return message.Fail(err.Error())
	// }
	block_timestamp_to, err := request.Parameters.GetUint64("block_timestamp_to")
	if err != nil {
		return message.Fail(err.Error())
	}
	// smartcontract_keys, err := request.Parameters.GetStringList("smartcontract_keys")
	// if err != nil {
	// return message.Fail(err.Error())
	// }
	page, err := request.Parameters.GetUint64("page")
	if err != nil {
		return message.Fail(err.Error())
	}
	if page == 0 {
		page = 1
	}
	limit, err := request.Parameters.GetUint64("limit")
	if err != nil {
		return message.Fail(err.Error())
	}
	if limit > 500 {
		return message.Fail("the limit exceeds 500. Please make it lower")
	}
	if limit == 0 {
		limit = 500
	}

	// smartcontracts := worker.GetSmartcontracts(evm_managers)

	// todo change the transaction
	/*if block_timestamp_to == 0 {
		block_timestamp_to, err = transaction.GetRecentBlockTimestamp(db, smartcontract_keys)
		if err != nil {
			return message.Fail("database error while trying to detect recent block timestamp: " + err.Error())
		}
	}*/

	/*transactions, err := transaction.TransactionGetAll(db, block_timestamp_from, block_timestamp_to, smartcontract_keys, page, limit)
	if err != nil {
		return message.Fail(err.Error())
	}*/

	var logs []*log.Log = []*log.Log{}
	/*
		if len(transactions) > 0 {
			txKeys := make([]string, len(transactions))
			for i, tx := range transactions {
				txKeys[i] = transaction.TransactionKey(tx.NetworkId, tx.Txid)
			}

			logs, err = log.GetLogsFromDb(db, txKeys)
			if err != nil {
				return message.Fail(err.Error())
			}
		}*/

	reply := message.Reply{
		Status: "OK",
		Parameters: key_value.New(map[string]interface{}{
			"logs":            data_type.ToMapList(logs),
			"block_timestamp": block_timestamp_to,
		}),
	}

	return reply
}
