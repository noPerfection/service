package message

import (
	"encoding/json"
	"fmt"
	"strings"
)

// The SDS Service will accepts the Request message.
type Request struct {
	Command    string
	Parameters map[string]interface{}
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
func (reply *Request) ToBytes() []byte {
	interfaces := reply.ToJSON()
	byt, err := json.Marshal(interfaces)
	if err != nil {
		fmt.Println("error while converting json into bytes", err)
		return []byte{}
	}

	return byt
}

// Convert Request message to the string
func (reply *Request) ToString() string {
	return string(reply.ToBytes())
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

	var dat map[string]interface{}

	decoder := json.NewDecoder(strings.NewReader(msg))
	decoder.UseNumber()

	if err := decoder.Decode(&dat); err != nil {
		return Request{}, err
	}

	command, err := GetString(dat, "command")
	if err != nil {
		return Request{}, err
	}
	parameters, err := GetMap(dat, "parameters")
	if err != nil {
		return Request{}, err
	}

	request := Request{
		Command:    command,
		Parameters: parameters,
	}

	return request, nil
}
