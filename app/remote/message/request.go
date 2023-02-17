package message

import (
	"github.com/blocklords/gosds/common/data_type/key_value"
)

// The SDS Service will accepts the Request message.
type Request struct {
	Command    string
	Parameters key_value.KeyValue
}

// Convert Request to JSON
func (request *Request) ToJSON() map[string]interface{} {
	return map[string]interface{}{
		"command":    request.Command,
		"parameters": request.Parameters,
	}
}

func (request *Request) CommandName() string {
	return request.Command
}

// Request message as a  sequence of bytes
func (request *Request) ToBytes() ([]byte, error) {
	return key_value.New(request.ToJSON()).ToBytes()
}

// Convert Request message to the string
func (request *Request) ToString() (string, error) {
	bytes, err := request.ToBytes()
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

// Messages from zmq concatenated
func ToString(msgs []string) string {
	msg := ""
	for _, v := range msgs {
		msg += v
	}
	return msg
}

// Parse the messages from zeromq into the Request
func ParseRequest(msgs []string) (Request, error) {
	msg := ToString(msgs)

	data, err := key_value.NewFromString(msg)

	if err != nil {
		return Request{}, err
	}

	command, err := data.GetString("command")
	if err != nil {
		return Request{}, err
	}
	parameters, err := data.GetKeyValue("parameters")
	if err != nil {
		return Request{}, err
	}

	request := Request{
		Command:    command,
		Parameters: parameters,
	}

	return request, nil
}
