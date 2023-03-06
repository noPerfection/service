/*Spaghetti transaction without method name and without clear input parameters*/
package transaction

import (
	"fmt"

	"github.com/blocklords/sds/common/data_type/key_value"
)

type Transaction struct {
	NetworkId      string  `json:"network_id"`
	BlockNumber    uint64  `json:"block_number"`
	BlockTimestamp uint64  `json:"block_timestamp"`
	Txid           string  `json:"txid"`     // txId column
	TxFrom         string  `json:"tx_from"`  // txFrom column
	TxTo           string  `json:"tx_to"`    // txTo column
	TxIndex        uint    `json:"tx_index"` // txIndex column
	Data           string  `json:"tx_data"`  // data columntext Data type
	Value          float64 `json:"tx_value"` // valueValue attached with transaction
}

// JSON string representation of the spaghetti.Transaction
func (t *Transaction) ToString() (string, error) {
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
