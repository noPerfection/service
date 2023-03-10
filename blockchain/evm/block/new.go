package block

import (
	"github.com/blocklords/sds/blockchain/event"
	"github.com/blocklords/sds/common/blockchain"
)

func NewBlock(network_id string, header blockchain.BlockHeader, logs []*event.RawLog) *Block {
	return &Block{
		NetworkId: network_id,
		Header:    header,
		RawLogs:   logs,
	}
}
