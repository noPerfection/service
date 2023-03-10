package blockchain

// Transaction Identifier in the blockchain
type TransactionKey struct {
	Id    string `json:"id"`    // txId column
	Index uint   `json:"index"` // txIndex column
}
