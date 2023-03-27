package command

import (
	"github.com/blocklords/sds/blockchain/event"
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type/key_value"
)

type FilterLog struct {
	BlockFrom blockchain.Number `json:"block_from"`
	Addresses []string          `json:"addresses"`
}

type LogFilterReply struct {
	BlockTo uint64         `json:"block_to"`
	RawLogs []event.RawLog `json:"raw_logs"`
}

func LogFilterParameters(parameters key_value.KeyValue) (FilterLog, error) {
	var filter FilterLog
	reply := LogFilterReply{}
	err := parameters.ToInterface(&reply)

	return filter, err
}
