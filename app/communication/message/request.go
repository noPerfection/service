package message

import (
	"fmt"

	"github.com/blocklords/sds/common/data_type/key_value"
)

// Request message sent by Client socket and accepted by Controller socket.
type Request struct {
	Command    string             `json:"command"`
	Parameters key_value.KeyValue `json:"parameters"`
	public_key string
}

// If the reply type is failure then
// THe message should be given too
func (request *Request) valid_command() error {
	if len(request.Command) == 0 {
		return fmt.Errorf("command is missing")
	}

	return nil
}

// ToBytes converts the message to the sequence of bytes
func (request *Request) ToBytes() ([]byte, error) {
	err := request.valid_command()
	if err != nil {
		return nil, fmt.Errorf("failed to validate command: %w", err)
	}

	kv, err := key_value.NewFromInterface(request)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize Request to key-value %v: %v", request, err)
	}

	bytes, err := kv.ToBytes()
	if err != nil {
		return nil, fmt.Errorf("kv.ToBytes: %w", err)
	}

	return bytes, nil
}

// For security; Work in Progress.
func (request *Request) SetPublicKey(public_key string) {
	request.public_key = public_key
}

// For security; Work in Progress.
func (request *Request) GetPublicKey() string {
	return request.public_key
}

// ToString the message
func (request *Request) ToString() (string, error) {
	bytes, err := request.ToBytes()
	if err != nil {
		return "", fmt.Errorf("request.ToBytes: %w", err)
	}

	return string(bytes), nil
}

// ToString into the single string the array of zeromq messages
func ToString(msgs []string) string {
	msg := ""
	for _, v := range msgs {
		msg += v
	}
	return msg
}

// ParseRequest from the zeromq messages
func ParseRequest(msgs []string) (Request, error) {
	msg := ToString(msgs)

	data, err := key_value.NewFromString(msg)
	if err != nil {
		return Request{}, fmt.Errorf("failed to convert message string %s to key-value: %v", msg, err)
	}

	var request Request
	err = data.ToInterface(&request)
	if err != nil {
		return Request{}, fmt.Errorf("failed to convert key-value %v to intermediate interface: %v", data, err)
	}

	// verify that data is not nil
	_, err = request.ToBytes()
	if err != nil {
		return Request{}, fmt.Errorf("failed to validate: %w", err)
	}

	return request, nil
}
