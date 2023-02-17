package handler

import (
	"github.com/blocklords/gosds/categorizer/transaction"

	"github.com/blocklords/gosds/app/remote/message"
	"github.com/blocklords/gosds/common/data_type"
	"github.com/blocklords/gosds/common/data_type/key_value"
	"github.com/blocklords/gosds/db"
)

// return transaction amount.
// this is done by SDS Publisher to count how many time it will fetch the transactions.
// the parameters are the same as for transaction_get_all command
func GetTransactionAmount(db *db.Database, request message.Request) message.Reply {
	block_timestamp_from, err := request.Parameters.GetUint64("block_timestamp_from")
	if err != nil {
		return message.Fail(err.Error())
	}
	block_timestamp_to, err := request.Parameters.GetUint64("block_timestamp_to")
	if err != nil {
		return message.Fail(err.Error())
	}
	smartcontract_keys, err := request.Parameters.GetStringList("smartcontract_keys")
	if err != nil {
		return message.Fail(err.Error())
	}

	transaction_amount, err := transaction.TransactionAmount(db, block_timestamp_from, block_timestamp_to, smartcontract_keys)
	if err != nil {
		return message.Fail(err.Error())
	}

	reply := message.Reply{
		Status: "OK",
		Parameters: key_value.New(map[string]interface{}{
			"transaction_amount": transaction_amount,
		}),
	}

	return reply
}

// return all transactions between block timestamps as well as for a list of smartcontract keys.
func GetTransactions(db *db.Database, request message.Request) message.Reply {
	block_timestamp_from, err := request.Parameters.GetUint64("block_timestamp_from")
	if err != nil {
		return message.Fail(err.Error())
	}
	block_timestamp_to, err := request.Parameters.GetUint64("block_timestamp_to")
	if err != nil {
		return message.Fail(err.Error())
	}
	smartcontract_keys, err := request.Parameters.GetStringList("smartcontract_keys")
	if err != nil {
		return message.Fail(err.Error())
	}
	page, err := request.Parameters.GetUint64("page")
	if err != nil {
		return message.Fail(err.Error())
	} else if page == 0 {
		page = 1
	}
	limit, err := request.Parameters.GetUint64("limit")
	if err != nil {
		return message.Fail(err.Error())
	} else if limit > 500 {
		return message.Fail("'limit' parameter can not exceed 500")
	} else if limit == 0 {
		limit = 500
	}

	transactions, err := transaction.TransactionGetAll(db, block_timestamp_from, block_timestamp_to, smartcontract_keys, page, limit)
	if err != nil {
		return message.Fail(err.Error())
	}

	reply := message.Reply{
		Status: "OK",
		Parameters: key_value.New(map[string]interface{}{
			"transactions": data_type.ToMapList(transactions),
		}),
	}

	return reply
}
