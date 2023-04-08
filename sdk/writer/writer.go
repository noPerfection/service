// Package writer defines the socket and message
// that user can use to send a transaction to the blockchain with
// smartcontract topic, not by its address.
package writer

import (
	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/common/topic"
)

type Writer struct {
	socket  *remote.ClientSocket // SDS Gateway host
	address string               // Account address granted for reading
}

func NewWriter(gatewaySocket *remote.ClientSocket, address string) *Writer {
	return &Writer{socket: gatewaySocket, address: address}
}

func (r *Writer) Write(t topic.Topic, args map[string]interface{}) message.Reply {
	if t.Level() != topic.FULL_LEVEL {
		return message.Fail(`Topic should contain method name`)
	}

	request := message.Request{
		Command: "smartcontract_write",
		Parameters: map[string]interface{}{
			"topic_string": t.ToString(topic.FULL_LEVEL),
			"arguments":    args,
			"address":      r.address,
		},
	}

	params, err := r.socket.RequestRemoteService(&request)
	if err != nil {
		return message.Fail(err.Error())
	}

	return message.Reply{Status: "OK", Message: "", Parameters: params}
}

func (r *Writer) AddToPool(t topic.Topic, args map[string]interface{}) message.Reply {
	if t.Level() != topic.FULL_LEVEL {
		return message.Fail(`Topic should contain method name`)
	}

	request := message.Request{
		Command: "pool_add",
		Parameters: map[string]interface{}{
			"topic_string": t.ToString(topic.FULL_LEVEL),
			"arguments":    args,
			"address":      r.address,
		},
	}

	params, err := r.socket.RequestRemoteService(&request)
	if err != nil {
		return message.Fail(err.Error())
	}

	return message.Reply{Status: "OK", Message: "", Parameters: params}
}
