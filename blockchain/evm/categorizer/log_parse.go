package categorizer

import (
	"fmt"

	"github.com/blocklords/gosds/app/remote"
	"github.com/blocklords/gosds/app/service"
	"github.com/blocklords/gosds/categorizer/event"
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

// Run the Smartcontract log parsing requests as a goroutine.
// The main worker function runs the subscriber socket.
// Running block range socket on another gourtine we can be sure about thread safety.
func LogParse(in chan RequestLogParse, out chan ReplyLogParse) {
	fmt.Println("running SDS Log requester as a goroutine")
	log_env, _ := service.New(service.LOG, service.REMOTE)
	categorizer_env, _ := service.New(service.CATEGORIZER, service.THIS)
	log_socket := remote.TcpRequestSocketOrPanic(log_env, categorizer_env)

	for {
		req := <-in
		fmt.Println(req.network_id, ".", req.address, ": request a log parse for data", req.data)

		log_name, outputs, err := event.RemoteLogParse(log_socket, req.network_id, req.address, req.data, req.topics)
		fmt.Println(req.network_id, ".", req.address, ": reply from SDS Log with a parsed log name", log_name)

		out <- ReplyLogParse{
			log_name: log_name,
			outputs:  outputs,
			err:      err,
		}
	}
}
