package block

import (
	"github.com/blocklords/gosds/spaghetti/log"
)

func NewBlock(network_id string, block_number uint64, block_timestamp uint64, logs []*log.Log) *Block {
	return &Block{
		NetworkId:      network_id,
		BlockNumber:    block_number,
		BlockTimestamp: block_timestamp,
		Logs:           logs,
	}
}
