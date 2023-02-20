package message

import (
	"fmt"

	"github.com/blocklords/gosds/common/data_type/key_value"
)

// SDS Service returns the reply. Anyone who sends a request to the SDS Service gets this message.
type Reply struct {
	Status     string             `json:"status"`
	Message    string             `json:"message"`
	Parameters key_value.KeyValue `json:"parameters"`
}

// Create a new Reply as a failure
// It accepts the error message that explains the reason of the failure.
func Fail(message string) Reply {
	return Reply{Status: "fail", Message: message, Parameters: key_value.Empty()}
}

// Is SDS Service returned a successful reply
func (r *Reply) IsOK() bool { return r.Status == "OK" }

// Convert the reply to the string format
func (reply *Reply) ToString() (string, error) {
	bytes, err := reply.ToBytes()
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

// Reply as a sequence of bytes
func (reply *Reply) ToBytes() ([]byte, error) {
	kv, err := key_value.NewFromInterface(reply)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize Reply to key-value %v: %v", reply, err)
	}

	return kv.ToBytes()
}

// Zeromq received raw strings converted to the Reply message.
func ParseReply(msgs []string) (Reply, error) {
	msg := ToString(msgs)
	data, err := key_value.NewFromString(msg)
	if err != nil {
		return Reply{}, err
	}

	return ParseJsonReply(data)
}

// Create 'Reply' message from a key value
func ParseJsonReply(dat key_value.KeyValue) (Reply, error) {
	var reply Reply
	err := dat.ToInterface(&reply)
	if err != nil {
		return Reply{}, fmt.Errorf("failed to serialize key-value to msg.Reply: %v", err)
	}
	return reply, nil
}
