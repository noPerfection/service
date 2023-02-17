package message

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/blocklords/gosds/common/data_type/key_value"
)

// SDS Service returns the reply. Anyone who sends a request to the SDS Service gets this message.
type Reply struct {
	Status  string
	Message string
	Params  map[string]interface{}
}

// Create a new Reply as a failure
// It accepts the error message that explains the reason of the failure.
func Fail(message string) Reply {
	return Reply{Status: "fail", Message: message, Params: map[string]interface{}{}}
}

// Is SDS Service returned a successful reply
func (r *Reply) IsOK() bool { return r.Status == "OK" }

// Convert to JSON
func (reply *Reply) ToJSON() map[string]interface{} {
	return map[string]interface{}{
		"status":  reply.Status,
		"message": reply.Message,
		"params":  reply.Params,
	}
}

// Convert the reply to the string format
func (reply *Reply) ToString() string {
	return string(reply.ToBytes())
}

// Reply as a sequence of bytes
func (reply *Reply) ToBytes() []byte {
	interfaces := reply.ToJSON()
	byt, err := json.Marshal(interfaces)
	if err != nil {
		return []byte{}
	}

	return byt
}

// Zeromq received raw strings converted to the Reply message.
func ParseReply(msgs []string) (Reply, error) {
	msg := ToString(msgs)
	var dat map[string]interface{}

	decoder := json.NewDecoder(strings.NewReader(msg))
	decoder.UseNumber()

	if err := decoder.Decode(&dat); err != nil {
		return Reply{}, err
	}

	return ParseJsonReply(dat)
}

// Create 'Reply' message from a json.
func ParseJsonReply(dat key_value.KeyValue) (Reply, error) {
	reply := Reply{}
	status, err := dat.GetString("status")
	if err != nil {
		return reply, err
	}
	if status != "fail" && status != "OK" {
		return reply, errors.New("the 'status' of the reply can be either 'fail' or 'OK'.")
	} else {
		reply.Status = status
	}

	message, err := dat.GetString("message")
	if err != nil {
		return reply, err
	} else {
		reply.Message = message
	}

	parameters, err := dat.GetMap("params")
	if err != nil {
		return reply, err
	} else {
		reply.Params = parameters
	}

	return reply, nil
}
