package message

import (
	"fmt"
	"time"

	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/google/uuid"
)

// Stack keeps the parameters of the message in the service.
type Stack struct {
	RequestTime    uint64 `json:"request_time"`
	ReplyTime      uint64 `json:"reply_time,omitempty"`
	Command        string `json:"command"`
	ServiceUrl     string `json:"service_url"`
	ServerName     string `json:"server_name"`
	ServerInstance string `json:"server_instance"`
}

// Request message sent by Client socket and accepted by Controller socket.
type Request struct {
	Uuid       string             `json:"uuid,omitempty"`
	Trace      []Stack            `json:"trace,omitempty"`
	Command    string             `json:"command"`
	Parameters key_value.KeyValue `json:"parameters"`
	publicKey  string
}

// If the reply type is failure then
// THe message should be given too
func (request *Request) validCommand() error {
	if len(request.Command) == 0 {
		return fmt.Errorf("command is missing")
	}

	return nil
}

// IsFirst returns true if the request has no trace
//
// For example, if the proxy will insert it.
func (request *Request) IsFirst() bool {
	return len(request.Trace) == 0
}

func (request *Request) AddRequestStack(serviceUrl string, serverName string, serverInstance string) {
	stack := Stack{
		RequestTime:    uint64(time.Now().UnixMicro()),
		ReplyTime:      0,
		Command:        request.Command,
		ServiceUrl:     serviceUrl,
		ServerName:     serverName,
		ServerInstance: serverInstance,
	}

	request.Trace = append(request.Trace, stack)
}

// Bytes converts the message to the sequence of bytes
func (request *Request) Bytes() ([]byte, error) {
	err := request.validCommand()
	if err != nil {
		return nil, fmt.Errorf("failed to validate command: %w", err)
	}

	kv, err := key_value.NewFromInterface(request)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize Request to key-value %v: %v", request, err)
	}

	bytes, err := kv.Bytes()
	if err != nil {
		return nil, fmt.Errorf("kv.Bytes: %w", err)
	}

	return bytes, nil
}

// SetPublicKey For security; Work in Progress.
func (request *Request) SetPublicKey(publicKey string) {
	request.publicKey = publicKey
}

// GetPublicKey For security; Work in Progress.
func (request *Request) GetPublicKey() string {
	return request.publicKey
}

// JoinMessages the message
func (request *Request) String() (string, error) {
	bytes, err := request.Bytes()
	if err != nil {
		return "", fmt.Errorf("request.Bytes: %w", err)
	}

	return string(bytes), nil
}

func (request *Request) SetUuid() {
	id := uuid.New()
	request.Uuid = id.String()
}

// Fail creates a new Reply as a failure
// It accepts the error message that explains the reason of the failure.
func (request *Request) Fail(message string) Reply {
	reply := Reply{
		Status:     FAIL,
		Message:    message,
		Parameters: key_value.Empty(),
		Uuid:       request.Uuid,
	}
	copy(reply.Trace, request.Trace)

	return reply
}

func (request *Request) Ok(parameters key_value.KeyValue) Reply {
	reply := Reply{
		Status:     FAIL,
		Message:    "",
		Parameters: parameters,
		Uuid:       request.Uuid,
	}
	copy(reply.Trace, request.Trace)

	return reply
}

// JoinMessages into the single string the array of zeromq messages
func JoinMessages(messages []string) string {
	msg := ""
	for _, v := range messages {
		msg += v
	}
	return msg
}

// ParseRequest from the zeromq messages
func ParseRequest(messages []string) (Request, error) {
	msg := JoinMessages(messages)

	data, err := key_value.NewFromString(msg)
	if err != nil {
		return Request{}, fmt.Errorf("failed to convert message string %s to key-value: %v", msg, err)
	}

	var request Request
	err = data.Interface(&request)
	if err != nil {
		return Request{}, fmt.Errorf("failed to convert key-value %v to intermediate interface: %v", data, err)
	}

	// verify that data is not nil
	_, err = request.Bytes()
	if err != nil {
		return Request{}, fmt.Errorf("failed to validate: %w", err)
	}

	return request, nil
}
