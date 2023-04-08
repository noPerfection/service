package handler

import (
	"github.com/blocklords/sds/blockchain/transaction"
)

// DeployedTransactionRequest is the required
// parameters in message.Request.Parameters
// for TRANSACTION_COMMAND
type DeployedTransactionRequest struct {
	NetworkId     string `json:"network_id"`
	TransactionId string `json:"transaction_id"`
}

// DeployedTransactionReply defines the keys and their
// types in the message.Reply.Parameters returned
// by the command handler for TRANSACTION_COMMAND
type DeployedTransactionReply struct {
	Raw transaction.RawTransaction `json:"transaction"`
}
