package transaction

import (
	"github.com/blocklords/gosds/categorizer/smartcontract"
	spaghetti_transaction "github.com/blocklords/gosds/spaghetti/transaction"
)

type Transaction struct {
	ID             string                 `json:"id,omitempty"`    // transaction key
	NetworkId      string                 `json:"network_id"`      // network
	Address        string                 `json:"address"`         // address
	BlockNumber    uint64                 `json:"block_number"`    // block number
	BlockTimestamp uint64                 `json:"block_timestamp"` // block timestamp
	Txid           string                 `json:"txid"`            // transaction
	TxIndex        uint                   `json:"tx_index"`        // transaction index
	TxFrom         string                 `json:"tx_from"`         // transaction from
	Method         string                 `json:"method"`          // method
	Args           map[string]interface{} `json:"arguments"`       // arguments
	Value          float64                `json:"value"`           // value
}

func TransactionKey(networkId string, txId string) string {
	return networkId + "." + txId
}

// Add the metadata such as transaction address from the Spaghetti transaction
func (transaction *Transaction) AddMetadata(spaghetti_transaction *spaghetti_transaction.Transaction) *Transaction {
	transaction.Txid = spaghetti_transaction.Txid
	transaction.TxIndex = spaghetti_transaction.TxIndex
	transaction.TxFrom = spaghetti_transaction.TxFrom
	transaction.Value = spaghetti_transaction.Value

	return transaction
}

// Add the smartcontract to which it belongs to from categorizer.Smartcontract
func (transaction *Transaction) AddSmartcontractData(smartcontract *smartcontract.Smartcontract) *Transaction {
	transaction.NetworkId = smartcontract.NetworkId
	transaction.Address = smartcontract.Address
	return transaction
}
