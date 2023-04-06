package handler

import (
	"github.com/blocklords/sds/common/blockchain"
)

// Used to fetch block number from
// blockchain client
//
// Used to inform a new block number to
// categorizer manager and old categorizer manager.
type RecentBlockHeaderRequest = blockchain.BlockHeader
type RecentBlockHeaderReply = blockchain.BlockHeader
