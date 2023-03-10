package block

import (
	"github.com/blocklords/sds/blockchain/event"
	"github.com/blocklords/sds/common/blockchain"
)

func NewBlock(network_id string, parameters blockchain.Block, logs []*event.RawLog) *Block {
	return &Block{
		NetworkId:  network_id,
		Parameters: parameters,
		RawLogs:    logs,
	}
}
