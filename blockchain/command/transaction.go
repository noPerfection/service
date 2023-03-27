package command

import (
	"fmt"

	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/blockchain/transaction"
	"github.com/blocklords/sds/common/data_type/key_value"
)

type Transaction struct {
	TransactionId string `json:"transaction_id"`
}

type TransactionReply struct {
	Raw transaction.RawTransaction `json:"transaction"`
}

func (request Transaction) Request(socket *remote.Socket) (*TransactionReply, error) {
	request_parameters, err := key_value.NewFromInterface(request)
	if err != nil {
		return nil, fmt.Errorf("conver parameters to: %w", err)
	}

	request_message := message.Request{
		Command:    TRANSACTION_COMMAND.String(),
		Parameters: request_parameters,
	}

	reply_parameters, err := socket.RequestRemoteService(&request_message)
	if err != nil {
		return nil, fmt.Errorf("socket.RequestRemoteService: %w", err)
	}

	reply := TransactionReply{}
	err = reply_parameters.ToInterface(&reply)
	return &reply, err
}

func (reply *TransactionReply) Reply() (message.Reply, error) {
	reply_parameters, err := key_value.NewFromInterface(reply)
	if err != nil {
		return message.Reply{}, fmt.Errorf("failed to encode reply: %w", err)
	}

	return message.Reply{
		Status:     message.OK,
		Message:    "",
		Parameters: reply_parameters,
	}, nil
}
