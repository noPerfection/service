// Package message contains the messages that services are exchanging
// Via the sockets.
//
// The message types are:
//
//   - Broadcast message sent by broadcast.Broadcast and received by the Subscriber.
//
//   - Request message sent by the client sockets to the remote services.
//
//   - Reply message sent back to clients by the Controller socket.
//
//   - SmartcontractDeveloperRequest message sent by client sockets to the Controller.
//     It's similar to the Request message, but includes the authentication parameters based on
//     Blockchain public/private keys.
//
//     This message is intended to be sent to the controller that has no CURVE authentication.
//     So the smartcontract developers can use their own private keys rather than keeping two
//     different types of keys.
//
// If the socket sent Request, and it will receive Reply.
package message

import (
	"fmt"
	"strings"

	"github.com/ahmetson/common-lib/data_type/key_value"
)

// Broadcast is the message that is submitted by Broadcast and received by Subscriber.
type Broadcast struct {
	// The parameters of the broadcast message and its status.
	Reply Reply `json:"reply"`
	// The topic to filter the incoming messages by the Subscriber.
	Topic string `json:"topic"`
}

// NewBroadcast creates the Broadcast from the fields.
func NewBroadcast(topic string, reply Reply) Broadcast {
	return Broadcast{
		Topic: topic,
		Reply: reply,
	}
}

// IsOK Broadcast was successful? Call it in the subscriber to verify the message state.
func (b *Broadcast) IsOK() bool { return b.Reply.IsOK() }

// ToBytes returns bytes representation of the Broadcast
func (b *Broadcast) ToBytes() []byte {
	kv, err := key_value.NewFromInterface(b)
	if err != nil {
		return []byte{}
	}

	bytes, _ := kv.Bytes()

	return bytes
}

// ParseBroadcast creates the Broadcast from the zeromq messages.
func ParseBroadcast(messages []string) (Broadcast, error) {
	msg := JoinMessages(messages)
	i := strings.Index(msg, "{")

	if i == -1 {
		return Broadcast{}, fmt.Errorf("invalid broadcast message %s, no distinction between topic and reply", msg)
	}

	broadcastRaw := msg[i:]

	dat, err := key_value.NewFromString(broadcastRaw)
	if err != nil {
		return Broadcast{}, fmt.Errorf("key_value.NewFromString: %w", err)
	}

	topic, err := dat.GetString("topic")
	if err != nil {
		return Broadcast{}, fmt.Errorf("broadcast.GetString(`topic`): %w", err)
	}

	rawReply, err := dat.GetKeyValue("reply")
	if err != nil {
		return Broadcast{}, fmt.Errorf("broadcast.GetKeyValue(`reply`): %w", err)
	}

	reply, err := ParseJsonReply(rawReply)
	if err != nil {
		return Broadcast{}, fmt.Errorf("ParseJsonReply: %w", err)
	}

	return Broadcast{Topic: topic, Reply: reply}, nil
}
