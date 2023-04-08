package handler

import (
	"github.com/blocklords/sds/blockchain/transaction"
)

// Transaction defines the required
// parameters in message.Request.Parameters for
// TRANSACTION_COMMAND
type Transaction struct {
	// TransactionId in the hex format with the '0x' prefix.
	TransactionId string `json:"transaction_id"`
}

// TransactionReply defines keys and value types
// of message.Reply.Parameters that are returned
// by controller after handling TRANSACTION_COMMAND
type TransactionReply struct {
	Raw transaction.RawTransaction `json:"transaction"`
}
