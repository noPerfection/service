package block

import (
	"github.com/blocklords/gosds/spaghetti/log"
	"github.com/blocklords/gosds/spaghetti/transaction"
)

func NewBlock(network_id string, block_number uint64, block_timestamp uint64, transactions []*transaction.Transaction, logs []*log.Log) *Block {
	return &Block{
		NetworkId:      network_id,
		BlockNumber:    block_number,
		BlockTimestamp: block_timestamp,
		Transactions:   transactions,
		Logs:           logs,
	}
}
