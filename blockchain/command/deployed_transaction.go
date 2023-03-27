package command

import (
	"github.com/blocklords/sds/blockchain/transaction"
)

type DeployedTransaction struct {
	NetworkId     string `json:"network_id"`
	TransactionId string `json:"transaction_id"`
}

type DeployedTransactionReply struct {
	Raw transaction.RawTransaction `json:"transaction"`
}
