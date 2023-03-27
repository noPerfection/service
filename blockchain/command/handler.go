package command

import (
	"fmt"

	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/blockchain/event"
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type/key_value"
)

const (
	FILTER_LOG_COMMAND = "log-filter"
)

type Command interface {
	Request(*remote.Socket) (interface{}, error)
}

type FilterLog struct {
	BlockFrom blockchain.Number `json:"block_from"`
	Addresses []string          `json:"addresses"`
}

type LogFilterReply struct {
	BlockTo uint64         `json:"block_to"`
	RawLogs []event.RawLog `json:"raw_logs"`
}

func (request FilterLog) Request(socket *remote.Socket) (*LogFilterReply, error) {
	request_parameters, err := key_value.NewFromInterface(request)
	if err != nil {
		return nil, fmt.Errorf("conver parameters to: %w", err)
	}

	request_message := message.Request{
		Command:    FILTER_LOG_COMMAND,
		Parameters: request_parameters,
	}

	reply_parameters, err := socket.RequestRemoteService(&request_message)
	if err != nil {
		return nil, fmt.Errorf("socket.RequestRemoteService: %w", err)
	}

	reply := LogFilterReply{}
	reply_parameters.ToInterface(&reply)
	return &reply, nil
}
