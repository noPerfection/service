package handler

import (
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type/key_value"
)

type RecentBlockRequest = key_value.KeyValue
type RecentBlockReply = blockchain.BlockHeader
