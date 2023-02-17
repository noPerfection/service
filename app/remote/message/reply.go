package message

import (
	"errors"

	"github.com/blocklords/gosds/common/data_type/key_value"
)

// SDS Service returns the reply. Anyone who sends a request to the SDS Service gets this message.
type Reply struct {
	Status     string
	Message    string
	Parameters key_value.KeyValue
}

// Create a new Reply as a failure
// It accepts the error message that explains the reason of the failure.
func Fail(message string) Reply {
	return Reply{Status: "fail", Message: message, Parameters: key_value.Empty()}
}

// Is SDS Service returned a successful reply
func (r *Reply) IsOK() bool { return r.Status == "OK" }

// Convert to JSON
func (reply *Reply) ToJSON() map[string]interface{} {
	return map[string]interface{}{
		"status":     reply.Status,
		"message":    reply.Message,
		"parameters": reply.Parameters,
	}
}

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
	return key_value.New(reply.ToJSON()).ToBytes()
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
	reply := Reply{}
	status, err := dat.GetString("status")
	if err != nil {
		return reply, err
	}
	if status != "fail" && status != "OK" {
		return reply, errors.New("the 'status' of the reply can be either 'fail' or 'OK'")
	} else {
		reply.Status = status
	}

	message, err := dat.GetString("message")
	if err != nil {
		return reply, err
	} else {
		reply.Message = message
	}

	parameters, err := dat.GetKeyValue("parameters")
	if err != nil {
		return reply, err
	} else {
		reply.Parameters = parameters
	}

	return reply, nil
}
