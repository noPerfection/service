package transaction

import (
	"github.com/blocklords/gosds/common/data_type/key_value"
)

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
func ParseTransaction(blob key_value.KeyValue) (*Transaction, error) {
	network_id, err := blob.GetString("network_id")
	if err != nil {
		return nil, err
	}
	address, err := blob.GetString("address")
	if err != nil {
		return nil, err
	}
	block_number, err := blob.GetUint64("block_number")
	if err != nil {
		return nil, err
	}
	block_timestamp, err := blob.GetUint64("block_timestamp")
	if err != nil {
		return nil, err
	}
	txid, err := blob.GetString("txid")
	if err != nil {
		return nil, err
	}
	tx_index, err := blob.GetUint64("tx_index")
	if err != nil {
		return nil, err
	}
	tx_from, err := blob.GetString("tx_from")
	if err != nil {
		return nil, err
	}
	method, err := blob.GetString("method")
	if err != nil {
		return nil, err
	}
	args, err := blob.GetKeyValue("arguments")
	if err != nil {
		return nil, err
	}

	value, err := blob.GetFloat64("value")
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
