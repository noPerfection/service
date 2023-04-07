package transaction

import (
	"errors"
	"fmt"

	"github.com/blocklords/sds/common/data_type/key_value"
)

// Parse the JSON into spaghetti.Transation
func New(parameters key_value.KeyValue) (*RawTransaction, error) {
	var transaction RawTransaction
	err := parameters.ToInterface(&transaction)
	if err != nil {
		return nil, fmt.Errorf("convert key value: %w", err)
	}

	err = transaction.Validate()
	if err != nil {
		return nil, fmt.Errorf("validation: %w", err)
	}

	return &transaction, nil
}

func NewTransactions(txs []interface{}) ([]*RawTransaction, error) {
	var transactions = make([]*RawTransaction, len(txs))
	for i, raw := range txs {
		if raw == nil {
			continue
		}
		map_log, ok := raw.(map[string]interface{})
		if !ok {
			return nil, errors.New("transaction is not a map")
		}
		transaction, err := New(map_log)
		if err != nil {
			return nil, err
		}
		transactions[i] = transaction
	}
	return transactions, nil
}
