package categorizer

import (
	"github.com/blocklords/gosds/message"
	"github.com/blocklords/gosds/remote"
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

// Add the metadata such as transaction address from the Spaghetti transaction
func (transaction *Transaction) AddMetadata(spaghetti_transaction *spaghetti_transaction.Transaction) *Transaction {
	transaction.Txid = spaghetti_transaction.Txid
	transaction.TxIndex = spaghetti_transaction.TxIndex
	transaction.TxFrom = spaghetti_transaction.TxFrom
	transaction.Value = spaghetti_transaction.Value

	return transaction
}

// Add the smartcontract to which it belongs to from categorizer.Smartcontract
func (transaction *Transaction) AddSmartcontractData(smartcontract *Smartcontract) *Transaction {
	transaction.NetworkId = smartcontract.NetworkId
	transaction.Address = smartcontract.Address
	return transaction
}

// Returns amount of transactions for the smartcontract keys within a certain block timestamp range.
func RemoteTransactionAmount(socket *remote.Socket, blockTimestampFrom int, blockTimestampTo int, smartcontractKeys []string) (int, error) {
	request := message.Request{
		Command: "transaction_amount",
		Parameters: map[string]interface{}{
			"block_timestamp_from": blockTimestampFrom,
			"block_timestamp_to":   blockTimestampTo,
			"smartcontract_keys":   smartcontractKeys,
		},
	}
	params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return 0, err
	}

	txAmount := int(params["transaction_amount"].(float64))

	return txAmount, nil
}

// Return transactions for smartcontract keys within a certain time range.
//
// It accepts a page and limit
func RemoteTransactions(socket *remote.Socket, blockTimestampFrom int, blockTimestampTo int, smartcontractKeys []string, page int, limit uint) ([]*Transaction, error) {
	request := message.Request{
		Command: "transaction_get_all",
		Parameters: map[string]interface{}{
			"block_timestamp_from": blockTimestampFrom,
			"block_timestamp_to":   blockTimestampTo,
			"smartcontract_keys":   smartcontractKeys,
			"page":                 page,
			"limit":                limit,
		},
	}

	params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return nil, err
	}

	raws := params["transactions"].([]interface{})
	transactions := make([]*Transaction, len(raws))
	for i, raw := range raws {
		transactions[i], err = ParseTransaction(raw.(map[string]interface{}))
		if err != nil {
			return nil, err
		}
	}

	return transactions, nil
}
