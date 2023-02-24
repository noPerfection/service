// EVM blockchain worker
package worker

import (
	"fmt"

	"github.com/blocklords/gosds/categorizer/abi"
	"github.com/blocklords/gosds/categorizer/log"
	"github.com/blocklords/gosds/categorizer/smartcontract"
	"github.com/blocklords/gosds/common/data_type/key_value"
	"github.com/blocklords/gosds/db"
	"github.com/blocklords/gosds/static/smartcontract/key"

	"github.com/blocklords/gosds/app/service"

	"github.com/blocklords/gosds/app/remote"
	"github.com/blocklords/gosds/app/remote/message"
	spaghetti_block "github.com/blocklords/gosds/spaghetti/block"
	spaghetti_log "github.com/blocklords/gosds/spaghetti/log"
)

type Worker struct {
	db *db.Database

	spaghetti_sub_socket      *remote.Socket
	spaghetti_block_range_in  chan RequestSpaghettiBlockRange
	spaghetti_block_range_out chan ReplySpaghettiBlockRange
	log_parse_in              chan RequestLogParse
	log_parse_out             chan ReplyLogParse

	smartcontract  *smartcontract.Smartcontract
	abi            *abi.Abi
	broadcast_chan chan message.Broadcast
}

type RequestSpaghettiBlockRange struct {
	network_id        string
	address           string
	block_number_from uint64
	block_number_to   uint64
}

type ReplySpaghettiBlockRange struct {
	timestamp uint64
	logs      []*spaghetti_log.Log
	err       error
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

func SpaghettiBlockRange(in chan RequestSpaghettiBlockRange, out chan ReplySpaghettiBlockRange) {
	fmt.Println("spaghetti block range requester runs as a gourtine")
	spaghetti_env, _ := service.New(service.SPAGHETTI, service.REMOTE)
	categorizer_env, _ := service.New(service.CATEGORIZER, service.THIS)

	spaghetti_socket := remote.TcpRequestSocketOrPanic(spaghetti_env, categorizer_env)

	for {
		fmt.Println("spaghetti block range requester received a command")
		req := <-in
		fmt.Println(req.network_id, ".", req.address, ": socket address", spaghetti_socket)
		fmt.Println(req.network_id, ".", req.address, ": request a block range from SDS Spaghetti for block range ", req.block_number_from, req.block_number_to)

		timestamp, logs,
			err := spaghetti_block.RemoteBlockRange(spaghetti_socket, req.network_id, req.address, req.block_number_from, req.block_number_to)
		fmt.Println(req.network_id, ".", req.address, ": timestamp of SDS Spaghetti reply", timestamp)

		out <- ReplySpaghettiBlockRange{
			timestamp: timestamp,
			logs:      logs,
			err:       err,
		}
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

// Print the log
func (worker *Worker) log_prefix() string {
	k := key.New(worker.smartcontract.NetworkId, worker.smartcontract.Address)
	return "categorizer " + k.ToString() + ": "
}

// Create a new worker
func NewWorker(db *db.Database, abi *abi.Abi, worker_smartcontract *smartcontract.Smartcontract, broadcast chan message.Broadcast, in chan RequestSpaghettiBlockRange, out chan ReplySpaghettiBlockRange, log_parse_in chan RequestLogParse, log_parse_out chan ReplyLogParse) *Worker {
	worker := Worker{
		smartcontract:             worker_smartcontract,
		broadcast_chan:            broadcast,
		db:                        db,
		spaghetti_block_range_in:  in,
		spaghetti_block_range_out: out,
		log_parse_in:              log_parse_in,
		log_parse_out:             log_parse_out,
		spaghetti_sub_socket:      nil,
		abi:                       abi,
	}

	return &worker
}

// broadcast the transactions and logs of the smartcontract.
func broadcast_block_categorization(worker *Worker, logs []map[string]interface{}) {
	// we assume that data is verified since the data comes from internal code.
	// not from outside.
	k := key.New(worker.smartcontract.NetworkId, worker.smartcontract.Address)
	broadcast_topic := k.ToString()

	new_reply := message.Reply{
		Status:  "OK",
		Message: "",
		Parameters: key_value.New(map[string]interface{}{
			"network_id":      worker.smartcontract.NetworkId,
			"block_number":    worker.smartcontract.CategorizedBlockNumber,
			"block_timestamp": worker.smartcontract.CategorizedBlockTimestamp,
			"address":         worker.smartcontract.Address,
			"logs":            logs,
		}),
	}
	new_broadcast := message.NewBroadcast(broadcast_topic, new_reply)

	worker.broadcast_chan <- new_broadcast
}

// Categorize the blocks for this smartcontract
func (worker *Worker) categorize(block_number uint64, block_timestamp uint64, logs []*spaghetti_log.Log) error {
	network_id := worker.smartcontract.NetworkId
	address := worker.smartcontract.Address

	broadcastLogs := make([]map[string]interface{}, 0)

	if len(logs) > 0 {
		for log_index := 0; log_index < len(logs); log_index++ {
			raw_log := logs[log_index]

			fmt.Println(worker.log_prefix(), "requesting parse of smartcontract log to SDS Log...")
			worker.log_parse_in <- RequestLogParse{
				network_id: network_id,
				address:    address,
				data:       raw_log.Data,
				topics:     raw_log.Topics,
			}
			log_reply := <-worker.log_parse_out
			fmt.Println(worker.log_prefix(), "reply received from SDS Log")
			if log_reply.err != nil {
				fmt.Println("abi.remote parse %w, we skip this log records", log_reply.err)
				continue
			}

			l := log.New(log_reply.log_name, log_reply.outputs).AddMetadata(raw_log).AddSmartcontractData(worker.smartcontract)
			err := log.Save(worker.db, l)
			if err != nil {
				return fmt.Errorf("emergency error. failed to create a log row in the database. this is an exception, that should not be. Consider fixing it. error message: " + err.Error())
			}

			log_kv, err := key_value.NewFromInterface(l)
			if err != nil {
				return fmt.Errorf("failed to serialize Log to key-value while trying to broadcast it %v: %v", l, err)
			}

			broadcastLogs = append(broadcastLogs, log_kv)
		}
	}

	fmt.Println(worker.log_prefix(), "categorization finished, update the block number to ", block_number)
	worker.smartcontract.SetBlockParameter(block_number, block_timestamp)
	broadcast_block_categorization(worker, broadcastLogs)
	err := smartcontract.SetSyncing(worker.db, worker.smartcontract, block_number, block_timestamp)

	return err
}
