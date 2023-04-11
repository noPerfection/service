package blockchain

import "fmt"

// Transaction Identifier in the blockchain
type TransactionKey struct {
	Id    string `json:"transaction_id"`    // txId column
	Index uint   `json:"transaction_index"` // txIndex column
}

// Validate the transaction key
func (tx TransactionKey) Validate() error {
	if len(tx.Id) == 0 {
		return fmt.Errorf("the Id is missing")
	}

	return nil
}
