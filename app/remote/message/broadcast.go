// The message package contains the message data types used between SDS Services.
//
// The message types are:
//   - Broadcast
//   - Request
//   - Reply
package message

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/blocklords/gosds/common/data_type/key_value"
)

// The broadcasters sends to all subscribers this message.
type Broadcast struct {
	Topic string
	reply Reply
}

// Convert to the format understood by the protocol
func (b *Broadcast) ToJSON() map[string]interface{} {
	return map[string]interface{}{
		"topic": b.Topic,
		"reply": b.reply.ToJSON(),
	}
}

// Broadcast as a string
func (b *Broadcast) ToString() string {
	return string(b.ToBytes())
}

// Broadcast as a sequence of bytes
func (reply *Broadcast) ToBytes() []byte {
	interfaces := reply.ToJSON()
	byt, err := json.Marshal(interfaces)
	if err != nil {
		return []byte{}
	}

	return byt
}

// Create a new broadcast
func NewBroadcast(topic string, reply Reply) Broadcast {
	return Broadcast{
		Topic: topic,
		reply: reply,
	}
}

// Broadcast's actual data for the subscriber
func (b *Broadcast) Reply() Reply {
	return b.reply
}

// Is OK
func (r *Broadcast) IsOK() bool { return r.reply.IsOK() }

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

	var dat key_value.KeyValue

	decoder := json.NewDecoder(strings.NewReader(broadcastRaw))
	decoder.UseNumber()

	if err := decoder.Decode(&dat); err != nil {
		return Broadcast{}, err
	}

	raw_reply, err := dat.GetMap("reply")
	if err != nil {
		return Broadcast{}, err
	}

	reply, err := ParseJsonReply(raw_reply)
	if err != nil {
		return Broadcast{}, err
	}

	return Broadcast{Topic: topic, reply: reply}, nil
}
