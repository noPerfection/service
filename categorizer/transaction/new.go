package transaction

import "github.com/blocklords/gosds/app/remote/message"

// Creates a new transaction, an incomplete function.
//
// This method should be called as:
//
//	categorizer.NewTransaction().AddMetadata().AddSmartcontractData()
func NewTransaction(method string, inputs map[string]interface{}, block_number uint64, block_timestamp uint64) *Transaction {
	return &Transaction{
		BlockNumber:    block_number,
		BlockTimestamp: block_timestamp,
		Method:         method,
		Args:           inputs,
	}
}

// Converts the JSON object into the corresponding transaction object.
func ParseTransaction(blob map[string]interface{}) (*Transaction, error) {
	network_id, err := message.GetString(blob, "network_id")
	if err != nil {
		return nil, err
	}
	address, err := message.GetString(blob, "address")
	if err != nil {
		return nil, err
	}
	block_number, err := message.GetUint64(blob, "block_number")
	if err != nil {
		return nil, err
	}
	block_timestamp, err := message.GetUint64(blob, "block_timestamp")
	if err != nil {
		return nil, err
	}
	txid, err := message.GetString(blob, "txid")
	if err != nil {
		return nil, err
	}
	tx_index, err := message.GetUint64(blob, "tx_index")
	if err != nil {
		return nil, err
	}
	tx_from, err := message.GetString(blob, "tx_from")
	if err != nil {
		return nil, err
	}
	method, err := message.GetString(blob, "method")
	if err != nil {
		return nil, err
	}
	args, err := message.GetMap(blob, "arguments")
	if err != nil {
		return nil, err
	}

	value, err := message.GetFloat64(blob, "value")
	if err != nil {
		return nil, err
	}

	return &Transaction{
		NetworkId:      network_id,
		Address:        address,
		BlockNumber:    block_number,
		BlockTimestamp: block_timestamp,
		Txid:           txid,
		TxIndex:        uint(tx_index),
		TxFrom:         tx_from,
		Method:         method,
		Args:           args,
		Value:          value,
	}, nil
}
