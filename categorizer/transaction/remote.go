package transaction

import (
	"github.com/blocklords/gosds/app/remote"
	"github.com/blocklords/gosds/app/remote/message"
)

// Returns amount of transactions for the smartcontract keys within a certain block timestamp range.
func RemoteTransactionAmount(socket *remote.Socket, blockTimestampFrom int, blockTimestampTo int, smartcontractKeys []string) (int, error) {
	request := message.Request{
		Command: "transaction_amount",
		Parameters: map[string]interface{}{
			"block_timestamp_from": blockTimestampFrom,
			"block_timestamp_to":   blockTimestampTo,
			"smartcontract_keys":   smartcontractKeys,
		},
	}
	params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return 0, err
	}

	txAmount := int(params["transaction_amount"].(float64))

	return txAmount, nil
}

// Return transactions for smartcontract keys within a certain time range.
//
// It accepts a page and limit
func RemoteTransactions(socket *remote.Socket, blockTimestampFrom int, blockTimestampTo int, smartcontractKeys []string, page int, limit uint) ([]*Transaction, error) {
	request := message.Request{
		Command: "transaction_get_all",
		Parameters: map[string]interface{}{
			"block_timestamp_from": blockTimestampFrom,
			"block_timestamp_to":   blockTimestampTo,
			"smartcontract_keys":   smartcontractKeys,
			"page":                 page,
			"limit":                limit,
		},
	}

	params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return nil, err
	}

	raws := params["transactions"].([]interface{})
	transactions := make([]*Transaction, len(raws))
	for i, raw := range raws {
		transactions[i], err = ParseTransaction(raw.(map[string]interface{}))
		if err != nil {
			return nil, err
		}
	}

	return transactions, nil
}
