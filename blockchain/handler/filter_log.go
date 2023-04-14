package handler

import (
	"github.com/blocklords/sds/blockchain/event"
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type/key_value"
)

// FilterLog defines the required parameters
// in message.Request.Parameters for FILTER_LOG_COMMAND
type FilterLog struct {
	BlockFrom blockchain.Number `json:"block_from"`
	Addresses []string          `json:"addresses"`
}

// LogFilterReply defines the fields and their
// types of message.Reply.Parameters that is
// returned by controller after handling FILTER_LOG_COMMAND
type LogFilterReply struct {
	BlockTo blockchain.Number `json:"block_to"`
	RawLogs []event.RawLog    `json:"raw_logs"`
}

// LogFilterParameters returns the message.Request.Parameters to FilterLog
func LogFilterParameters(parameters key_value.KeyValue) (FilterLog, error) {
	var filter FilterLog
	reply := LogFilterReply{}
	err := parameters.ToInterface(&reply)

	return filter, err
}
