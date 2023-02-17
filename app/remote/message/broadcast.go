// The message package contains the message data types used between SDS Services.
//
// The message types are:
//   - Broadcast
//   - Request
//   - Reply
package message

import (
	"errors"
	"strings"

	"github.com/blocklords/gosds/common/data_type/key_value"
)

// The broadcasters sends to all subscribers this message.
type Broadcast struct {
	Topic string `json:"topic"`
	Reply Reply  `json:"reply"`
}

// Create a new broadcast
func NewBroadcast(topic string, reply Reply) Broadcast {
	return Broadcast{
		Topic: topic,
		Reply: reply,
	}
}

// Is OK
func (r *Broadcast) IsOK() bool { return r.Reply.IsOK() }

// Reply as a sequence of bytes
func (b *Broadcast) ToBytes() []byte {
	kv, err := key_value.NewFromInterface(b)
	if err != nil {
		return []byte{}
	}

	bytes, _ := kv.ToBytes()

	return bytes
}

// Parse the zeromq messages into a broadcast
func ParseBroadcast(msgs []string) (Broadcast, error) {
	msg := ""
	for _, v := range msgs {
		msg += v
	}
	i := strings.Index(msg, "{")

	if i == -1 {
		return Broadcast{}, errors.New("invalid message, no distinction between topic and reply")
	}

	topic := msg[:i]
	broadcastRaw := msg[i:]

	dat, err := key_value.NewFromString(broadcastRaw)
	if err != nil {
		return Broadcast{}, err
	}

	raw_reply, err := dat.GetKeyValue("reply")
	if err != nil {
		return Broadcast{}, err
	}

	reply, err := ParseJsonReply(raw_reply)
	if err != nil {
		return Broadcast{}, err
	}

	return Broadcast{Topic: topic, Reply: reply}, nil
}
