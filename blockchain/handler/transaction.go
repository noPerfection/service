package handler

import (
	"github.com/blocklords/sds/blockchain/transaction"
)

type Transaction struct {
	TransactionId string `json:"transaction_id"`
}

type TransactionReply struct {
	Raw transaction.RawTransaction `json:"transaction"`
}
