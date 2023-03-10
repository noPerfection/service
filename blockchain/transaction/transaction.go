/*Spaghetti transaction without method name and without clear input parameters*/
package transaction

import (
	"fmt"

	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/static/smartcontract/key"
)

type RawTransaction struct {
	Key            key.Key                   `json:"key"`
	Block          blockchain.Block          `json:"block"`
	TransactionKey blockchain.TransactionKey `json:"transaction_key"`
	From           string                    `json:"transaction_from"`            // txFrom column
	Data           string                    `json:"transaction_data,omitempty"`  // data columntext Data type
	Value          float64                   `json:"transaction_value,omitempty"` // valueValue attached with transaction
}

// JSON string representation of the spaghetti.Transaction
func (t *RawTransaction) ToString() (string, error) {
	kv, err := key_value.NewFromInterface(t)
	if err != nil {
		return "", fmt.Errorf("failed to serialize spaghetti transaction to intermediate key-value %v: %v", t, err)
	}

	bytes, err := kv.ToBytes()
	if err != nil {
		return "", fmt.Errorf("failed to serialize intermediate key-value to string %v: %v", t, err)
	}

	return string(bytes), nil
}
