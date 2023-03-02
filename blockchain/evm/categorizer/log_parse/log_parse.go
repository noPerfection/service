package log_parse

import (
	"fmt"
	"log"
	"sync"

	"github.com/blocklords/gosds/app/remote"
	"github.com/blocklords/gosds/app/remote/message"
	"github.com/blocklords/gosds/app/service"
	"github.com/blocklords/gosds/categorizer/event"
	"github.com/blocklords/gosds/common/data_type/key_value"

	zmq "github.com/pebbe/zmq4"
)

type RequestLogParse struct {
	network_id string
	address    string
	data       string
	topics     []string
}

type ReplyLogParse struct {
	log_name string
	outputs  map[string]interface{}
	err      error
}

const LOG_PARSE_URL = "inproc://spaghetti_evm_log_parses"

// EVM based categorizers calls this function to connect to the Log Parse process.
// Log Parse on its own hand connects to the remote SDS Log service to decode the event log
func ParseLog(sock *remote.Socket, network_id string, address string, data string, topics []string) (string, key_value.KeyValue, error) {
	request := message.Request{
		Command: "parse-log",
		Parameters: map[string]interface{}{
			"network_id": network_id,
			"address":    address,
			"data":       data,
			"topics":     topics,
		},
	}

	parameters, err := sock.RequestRemoteService(&request)
	if err != nil {
		return "", nil, fmt.Errorf("socket.RequestRemoteService: %w", err)
	}

	log_name, _ := parameters.GetString("log_name")
	outputs, _ := parameters.GetKeyValue("outputs")

	return log_name, outputs, nil
}

// Run the Smartcontract log parsing requests as a goroutine.
// The main worker function runs the subscriber socket.
// Running block range socket on another gourtine we can be sure about thread safety.
func RunLogParse() {

	internal_sock, err := zmq.NewSocket(zmq.REP)
	if err != nil {
		log.Fatalf("trying to create evm log parser socket for network id %s: %v", LOG_PARSE_URL, err)
	}

	if err := internal_sock.Bind(LOG_PARSE_URL); err != nil {
		log.Fatalf("trying to create evm log parser %s: %v", LOG_PARSE_URL, err)
	}

	fmt.Println("running SDS Log requester as a goroutine")

	var mu sync.Mutex

	log_env, _ := service.New(service.LOG, service.REMOTE)
	categorizer_env, _ := service.New(service.CATEGORIZER, service.THIS)
	log_socket := remote.TcpRequestSocketOrPanic(log_env, categorizer_env)

	for {
		// Wait for reply.
		msgs, _ := internal_sock.RecvMessage(0)
		request, _ := message.ParseRequest(msgs)

		network_id, _ := request.Parameters.GetString("network_id")
		address, _ := request.Parameters.GetString("address")
		data, _ := request.Parameters.GetString("data")
		topics, _ := request.Parameters.GetStringList("topics")

		mu.Lock()
		log_name, outputs, err := event.RemoteLogParse(log_socket, network_id, address, data, topics)
		mu.Unlock()

		var reply message.Reply
		if err != nil {
			reply = message.Fail(fmt.Sprintf("event.RemoteLogParse(network_id=%s, address=%s, data=%s): %s", network_id, address, data, err.Error()))
		} else {
			reply = message.Reply{
				Status:     "OK",
				Message:    "",
				Parameters: key_value.Empty().Set("log_name", log_name).Set("outputs", outputs),
			}
		}

		mu.Lock()
		reply_string, err := reply.ToString()
		if err != nil {
			if _, err := internal_sock.SendMessage(err.Error()); err != nil {
				mu.Unlock()
				log.Fatalf("reply.ToString error to send message by %s error: %s", LOG_PARSE_URL, err.Error())
			}
		} else {
			if _, err := internal_sock.SendMessage(reply_string); err != nil {
				mu.Unlock()
				log.Fatalf("failed to reply by %s: %s", LOG_PARSE_URL, err.Error())
			}
		}
		mu.Unlock()
	}
}
