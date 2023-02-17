package block

import (
	"github.com/blocklords/gosds/app/remote"
	"github.com/blocklords/gosds/app/remote/message"
	"github.com/blocklords/gosds/common/data_type/key_value"

	"github.com/blocklords/gosds/spaghetti/log"
	"github.com/blocklords/gosds/spaghetti/transaction"
)

// Returns the earliest number in the cache for a given network id
func RemoteBlockNumberCached(socket *remote.Socket, network_id string) (uint64, uint64, error) {
	// Send hello.
	request := message.Request{
		Command: "block_get_cached_number",
		Parameters: map[string]interface{}{
			"network_id": network_id,
		},
	}

	raw_parameters, err := socket.RequestRemoteService(&request)
	if err != nil {
		return 0, 0, err
	}
	parameters := key_value.New(raw_parameters)

	block_number, err := parameters.GetUint64("block_number")
	if err != nil {
		return 0, 0, err
	}
	block_timestamp, err := parameters.GetUint64("block_timestamp")
	if err != nil {
		return 0, 0, err
	}

	return block_number, block_timestamp, nil
}

// Returns the block minted time from SDS Spaghetti
func RemoteBlockMintedTime(socket *remote.Socket, networkId string, blockNumber uint64) (uint64, error) {
	// Send hello.
	request := message.Request{
		Command: "block_get_timestamp",
		Parameters: map[string]interface{}{
			"network_id":   networkId,
			"block_number": blockNumber,
		},
	}

	raw_parameters, err := socket.RequestRemoteService(&request)
	if err != nil {
		return 0, err
	}
	parameters := key_value.New(raw_parameters)

	return parameters.GetUint64("block_timestamp")
}

func RemoteBlockRange(socket *remote.Socket, networkId string, address string, from uint64, to uint64) (uint64, []*transaction.Transaction, []*log.Log, error) {
	request := message.Request{
		Command: "block_get_range",
		Parameters: map[string]interface{}{
			"block_number_from": from,
			"block_number_to":   to,
			"to":                address,
			"network_id":        networkId,
		},
	}

	raw_parameters, err := socket.RequestRemoteService(&request)
	if err != nil {
		return 0, nil, nil, err
	}
	parameters := key_value.New(raw_parameters)

	timestamp, err := parameters.GetUint64("timestamp")
	if err != nil {
		return 0, nil, nil, err
	}

	raw_transactions, err := parameters.GetKeyValueList("transactions")
	if err != nil {
		return 0, nil, nil, err
	}

	raw_logs, err := parameters.GetKeyValueList("logs")
	if err != nil {
		return 0, nil, nil, err
	}

	transactions := make([]*transaction.Transaction, len(raw_transactions))
	for i, raw := range raw_transactions {
		tx, err := transaction.NewFromMap(raw)
		if err != nil {
			return 0, nil, nil, err
		}
		transactions[i] = tx
	}

	logs := make([]*log.Log, len(raw_logs))
	for i, raw := range raw_logs {
		l, err := log.New(raw)
		if err != nil {
			return 0, nil, nil, err
		}
		logs[i] = l
	}

	return timestamp, transactions, logs, nil
}

// Returns the remote block information
// The address parameter is optional (make it a blank string)
// In that case SDS Spaghetti will return block with all transactions and logs.
func RemoteBlock(socket *remote.Socket, network_id string, block_number uint64, address string) (bool, *Block, error) {
	request := message.Request{
		Command: "block_get",
		Parameters: map[string]interface{}{
			"block_number": block_number,
			"network_id":   network_id,
			"to":           address,
		},
	}

	raw_parameters, err := socket.RequestRemoteService(&request)
	if err != nil {
		return false, nil, err
	}
	parameters := key_value.New(raw_parameters)

	cached, err := parameters.GetBoolean("cached")
	if err != nil {
		return false, nil, err
	}

	timestamp, err := parameters.GetUint64("timestamp")
	if err != nil {
		return false, nil, err
	}

	raw_transactions, err := parameters.GetKeyValueList("transactions")
	if err != nil {
		return false, nil, err
	}

	raw_logs, err := parameters.GetKeyValueList("logs")
	if err != nil {
		return false, nil, err
	}

	transactions := make([]*transaction.Transaction, len(raw_transactions))
	for i, raw := range raw_transactions {
		tx, err := transaction.NewFromMap(raw)
		if err != nil {
			return false, nil, err
		}
		transactions[i] = tx
	}

	logs := make([]*log.Log, len(raw_logs))
	for i, raw := range raw_logs {
		l, err := log.New(raw)
		if err != nil {
			return false, nil, err
		}
		logs[i] = l
	}

	return cached, NewBlock(network_id, block_number, timestamp, transactions, logs), err
}
