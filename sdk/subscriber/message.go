package subscriber

import (
	"github.com/blocklords/sds/indexer/event"
	"github.com/blocklords/sds/service/communication/message"
)

type Parameters struct {
	Logs          []*event.Log `json:"logs"`
	TimestampFrom uint64       `json:"timestamp_from"`
	TimestampTo   uint64       `json:"timestamp_to"`
}

type Message struct {
	Status     message.ReplyStatus `json:"status"`
	Message    string              `json:"message,omitempty"`
	Parameters Parameters
}

func NewErrorMessage(error string) Message {
	return Message{Status: message.FAIL, Message: error, Parameters: Parameters{}}
}

func NewMessage(logs []*event.Log, timestamp_from uint64, timestamp_to uint64) Message {
	parameters := Parameters{Logs: logs, TimestampFrom: timestamp_from, TimestampTo: timestamp_to}
	return Message{Status: message.OK, Parameters: parameters}
}
