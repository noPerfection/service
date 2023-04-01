/*Spaghetti transaction without method name and without clear input parameters*/
package transaction

import (
	"fmt"

	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/smartcontract_key"
)

// Blockchain agnostic transaction
// The transaction information
//
// The SmartcontractKey field keeps the
// network id where this transaction occured
// as well as the receiver/smartcontract address
type RawTransaction struct {
	SmartcontractKey smartcontract_key.Key     `json:"smartcontract_key"`
	BlockHeader      blockchain.BlockHeader    `json:"block_header"`
	TransactionKey   blockchain.TransactionKey `json:"transaction_key"`
	From             string                    `json:"transaction_from"`            // txFrom column
	Data             string                    `json:"transaction_data,omitempty"`  // data columntext Data type
	Value            float64                   `json:"transaction_value,omitempty"` // valueValue attached with transaction
}

func (t *RawTransaction) validate() error {
	if len(t.SmartcontractKey.Address) == 0 {
		return fmt.Errorf("smartcontract_key.address is empty")
	}
	if len(t.SmartcontractKey.NetworkId) == 0 {
		return fmt.Errorf("smartcontract_key.network_id is empty")
	}
	if t.BlockHeader.Number.Value() == 0 {
		return fmt.Errorf("block_header.block_number is 0")
	}
	if t.BlockHeader.Timestamp.Value() == 0 {
		return fmt.Errorf("block_header.block_timestamp is 0")
	}
	if len(t.TransactionKey.Id) == 0 {
		return fmt.Errorf("transaction_key.id is 0")
	}
	if len(t.From) == 0 {
		return fmt.Errorf("'from' parameter is empty")
	}
	return nil
}

// JSON string representation of the spaghetti.Transaction
func (t *RawTransaction) ToString() (string, error) {
	err := t.validate()
	if err != nil {
		return "", fmt.Errorf("validation: %w", err)
	}
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
