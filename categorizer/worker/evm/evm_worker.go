// EVM blockchain worker
package evm

import (
	"fmt"

	"github.com/blocklords/gosds/categorizer/abi"
	"github.com/blocklords/gosds/categorizer/log"
	"github.com/blocklords/gosds/categorizer/smartcontract"
	"github.com/blocklords/gosds/categorizer/worker"
	"github.com/blocklords/gosds/common/data_type/key_value"

	"github.com/blocklords/gosds/app/service"

	"github.com/blocklords/gosds/app/remote"
	spaghetti_log "github.com/blocklords/gosds/spaghetti/log"
)

// Wrapper around the gosds/categorizer/worker.Worker
// For EVM based smartcontracts
type EvmWorker struct {
	abi *abi.Abi

	spaghetti_sub_socket *remote.Socket
	log_parse_in         chan RequestLogParse
	log_parse_out        chan ReplyLogParse

	parent *worker.Worker
}

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

// Wraps the Worker with the EVM related data and returns the wrapped Worker as EvmWorker
func New(parent *worker.Worker, abi *abi.Abi, log_parse_in chan RequestLogParse, log_parse_out chan ReplyLogParse) *EvmWorker {
	return &EvmWorker{
		abi:           abi,
		log_parse_in:  log_parse_in,
		log_parse_out: log_parse_out,
		parent:        parent,
	}
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

		log_name, outputs, err := log.RemoteLogParse(log_socket, req.network_id, req.address, req.data, req.topics)
		fmt.Println(req.network_id, ".", req.address, ": reply from SDS Log with a parsed log name", log_name)

		out <- ReplyLogParse{
			log_name: log_name,
			outputs:  outputs,
			err:      err,
		}
	}
}

func broadcast_block_categorization(worker *EvmWorker, logs []map[string]interface{}) {
	// // we assume that data is verified since the data comes from internal code.
	// // not from outside.
	// k := key.New(worker.Smartcontract.NetworkId, worker.Smartcontract.Address)
	// broadcast_topic := k.ToString()

	// new_reply := message.Reply{
	// 	Status:  "OK",
	// 	Message: "",
	// 	Parameters: key_value.New(map[string]interface{}{
	// 		"network_id":      worker.Smartcontract.NetworkId,
	// 		"block_number":    worker.Smartcontract.CategorizedBlockNumber,
	// 		"block_timestamp": worker.Smartcontract.CategorizedBlockTimestamp,
	// 		"address":         worker.Smartcontract.Address,
	// 		"logs":            logs,
	// 	}),
	// }
	// new_broadcast := message.NewBroadcast(broadcast_topic, new_reply)

	// worker.broadcast_chan <- new_broadcast
}

// Categorize the blocks for this smartcontract
func (worker *EvmWorker) categorize(logs []*spaghetti_log.Log) (uint64, error) {
	network_id := worker.parent.Smartcontract.NetworkId
	address := worker.parent.Smartcontract.Address

	broadcastLogs := make([]map[string]interface{}, 0)

	var block_number uint64 = worker.parent.Smartcontract.CategorizedBlockNumber
	var block_timestamp uint64 = worker.parent.Smartcontract.CategorizedBlockTimestamp

	if len(logs) > 0 {
		for log_index := 0; log_index < len(logs); log_index++ {
			raw_log := logs[log_index]

			fmt.Println(worker.parent.Prefix(), "requesting parse of smartcontract log to SDS Log...")
			worker.log_parse_in <- RequestLogParse{
				network_id: network_id,
				address:    address,
				data:       raw_log.Data,
				topics:     raw_log.Topics,
			}
			log_reply := <-worker.log_parse_out
			fmt.Println(worker.parent.Prefix(), "reply received from SDS Log")
			if log_reply.err != nil {
				fmt.Println("abi.remote parse %w, we skip this log records", log_reply.err)
				continue
			}

			l := log.New(log_reply.log_name, log_reply.outputs).AddMetadata(raw_log).AddSmartcontractData(worker.parent.Smartcontract)
			err := log.Save(worker.parent.Db, l)
			if err != nil {
				return 0, fmt.Errorf("emergency error. failed to create a log row in the database. this is an exception, that should not be. Consider fixing it. error message: " + err.Error())
			}

			if l.BlockNumber > block_number {
				block_number = l.BlockNumber
				block_timestamp = l.BlockTimestamp
			}

			log_kv, err := key_value.NewFromInterface(l)
			if err != nil {
				return 0, fmt.Errorf("failed to serialize Log to key-value while trying to broadcast it %v: %v", l, err)
			}

			broadcastLogs = append(broadcastLogs, log_kv)
		}
	}

	fmt.Println(worker.parent.Prefix(), "categorization finished, update the block number to ", block_number)
	worker.parent.Smartcontract.SetBlockParameter(block_number, block_timestamp)
	broadcast_block_categorization(worker, broadcastLogs)
	err := smartcontract.SetSyncing(worker.parent.Db, worker.parent.Smartcontract, block_number, block_timestamp)

	return block_number, err
}
