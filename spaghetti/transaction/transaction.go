/*Spaghetti transaction without method name and without clear input parameters*/
package transaction

import (
	"encoding/json"
)

type Transaction struct {
	NetworkId      string
	BlockNumber    uint64
	BlockTimestamp uint64
	Txid           string // txId column
	TxFrom         string
	TxTo           string
	TxIndex        uint
	Data           string  // text Data type
	Value          float64 // Value attached with transaction
}

// JSON representation of the spaghetti.Transaction
func (b *Transaction) ToJSON() map[string]interface{} {
	return map[string]interface{}{
		"network_id":      b.NetworkId,
		"block_number":    b.BlockNumber,
		"block_timestamp": b.BlockTimestamp,
		"Txid":            b.Txid,
		"tx_from":         b.TxFrom,
		"tx_to":           b.TxTo,
		"tx_index":        b.TxIndex,
		"tx_Data":         b.Data,
		"tx_Value":        b.Value,
	}
}

// JSON string representation of the spaghetti.Transaction
func (b *Transaction) ToString() string {
	interfaces := b.ToJSON()
	byt, err := json.Marshal(interfaces)
	if err != nil {
		return ""
	}

	return string(byt)
}
