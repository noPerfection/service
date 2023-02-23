package transaction

import (
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/blocklords/gosds/common/data_type/key_value"
	eth_types "github.com/ethereum/go-ethereum/core/types"
)

func New(network_id string, block_number uint64, transaction_index uint, tx *eth_types.Transaction) (*Transaction, error) {
	msg, err := tx.AsMessage(eth_types.LatestSignerForChainID(tx.ChainId()), tx.GasPrice())
	if err != nil {
		fmt.Println("Error parsing transaction. Failed to get 'From' field. Error: ", err)
		return nil, err
	}

	bigValue := wei_to_ether(tx.Value())
	value, _ := bigValue.Float64()
	toAddr := tx.To()
	to := ""
	if toAddr != nil {
		to = toAddr.Hex()
	}

	return &Transaction{
		NetworkId:      network_id,
		BlockNumber:    block_number,
		BlockTimestamp: 0,
		Txid:           tx.Hash().String(),
		TxIndex:        transaction_index,
		TxFrom:         msg.From().Hex(),
		TxTo:           to,
		Data:           hex.EncodeToString(tx.Data()),
		Value:          value,
	}, nil
}

// Parse the JSON into spaghetti.Transation
func NewFromMap(parameters key_value.KeyValue) (*Transaction, error) {
	network_id, err := parameters.GetString("network_id")
	if err != nil {
		return nil, err
	}
	block_number, err := parameters.GetUint64("block_number")
	if err != nil {
		return nil, err
	}
	block_timestamp, err := parameters.GetUint64("block_timestamp")
	if err != nil {
		return nil, err
	}
	Txid, err := parameters.GetString("txid")
	if err != nil {
		return nil, err
	}
	tx_index, err := parameters.GetUint64("tx_index")
	if err != nil {
		return nil, err
	}
	tx_from, err := parameters.GetString("tx_from")
	if err != nil {
		return nil, err
	}
	tx_to, err := parameters.GetString("tx_to")
	if err != nil {
		return nil, err
	}
	tx_Data, err := parameters.GetString("tx_data")
	if err != nil {
		return nil, err
	}
	Value, err := parameters.GetFloat64("tx_value")
	if err != nil {
		return nil, err
	}

	return &Transaction{
		NetworkId:      network_id,
		BlockNumber:    block_number,
		BlockTimestamp: block_timestamp,
		Txid:           Txid,
		TxIndex:        uint(tx_index),
		TxFrom:         tx_from,
		TxTo:           tx_to,
		Data:           tx_Data,
		Value:          Value,
	}, nil
}

func NewTransactions(txs []interface{}) ([]*Transaction, error) {
	var transactions []*Transaction = make([]*Transaction, len(txs))
	for i, raw := range txs {
		if raw == nil {
			continue
		}
		map_log, ok := raw.(map[string]interface{})
		if !ok {
			return nil, errors.New("transaction is not a map")
		}
		transaction, err := NewFromMap(map_log)
		if err != nil {
			return nil, err
		}
		transactions[i] = transaction
	}
	return transactions, nil
}
