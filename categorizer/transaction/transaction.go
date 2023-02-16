package transaction

import (
	"github.com/blocklords/gosds/categorizer/smartcontract"
	spaghetti_transaction "github.com/blocklords/gosds/spaghetti/transaction"
)

type Transaction struct {
	ID             string // transaction key
	NetworkId      string
	Address        string
	BlockNumber    uint64
	BlockTimestamp uint64
	Txid           string
	TxIndex        uint
	TxFrom         string
	Method         string
	Args           map[string]interface{}
	Value          float64
}

func TransactionKey(networkId string, txId string) string {
	return networkId + "." + txId
}

func (b *Transaction) ToJSON() map[string]interface{} {
	i := map[string]interface{}{}
	i["network_id"] = b.NetworkId
	i["address"] = b.Address
	i["block_number"] = b.BlockNumber
	i["block_timestamp"] = b.BlockTimestamp
	i["txid"] = b.Txid
	i["tx_index"] = b.TxIndex
	i["tx_from"] = b.TxFrom
	i["method"] = b.Method
	i["arguments"] = b.Args
	i["value"] = b.Value
	return i
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
