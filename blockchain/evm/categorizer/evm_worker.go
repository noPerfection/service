// EVM blockchain worker
package categorizer

import (
	"fmt"

	"github.com/blocklords/gosds/blockchain/evm/abi"
	"github.com/blocklords/gosds/categorizer/log"
	"github.com/blocklords/gosds/categorizer/smartcontract"

	"github.com/blocklords/gosds/app/service"

	"github.com/blocklords/gosds/app/remote"
	spaghetti_log "github.com/blocklords/gosds/blockchain/log"
)

// Wrapper around the gosds/categorizer/worker.Worker
// For EVM based smartcontracts
type EvmWorker struct {
	abi *abi.Abi

	log_parse_in  chan RequestLogParse
	log_parse_out chan ReplyLogParse

	smartcontract *smartcontract.Smartcontract
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
func New(sm *smartcontract.Smartcontract, abi *abi.Abi) *EvmWorker {
	return &EvmWorker{
		abi:           abi,
		smartcontract: sm,
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

// Categorize the blocks for this smartcontract
func (worker *EvmWorker) categorize(logs []*spaghetti_log.Log) (uint64, error) {
	network_id := worker.smartcontract.NetworkId
	address := worker.smartcontract.Address

	var block_number uint64 = worker.smartcontract.CategorizedBlockNumber
	var block_timestamp uint64 = worker.smartcontract.CategorizedBlockTimestamp

	if len(logs) > 0 {
		for log_index := 0; log_index < len(logs); log_index++ {
			raw_log := logs[log_index]

			fmt.Println("requesting parse of smartcontract log to SDS Log...", raw_log, worker.smartcontract)
			worker.log_parse_in <- RequestLogParse{
				network_id: network_id,
				address:    address,
				data:       raw_log.Data,
				topics:     raw_log.Topics,
			}
			log_reply := <-worker.log_parse_out
			fmt.Println("reply received from SDS Log")
			if log_reply.err != nil {
				fmt.Println("abi.remote parse %w, we skip this log records", log_reply.err)
				continue
			}

			l := log.New(log_reply.log_name, log_reply.outputs).AddMetadata(raw_log).AddSmartcontractData(worker.smartcontract)

			if l.BlockNumber > block_number {
				block_number = l.BlockNumber
				block_timestamp = l.BlockTimestamp
			}
		}
	}

	fmt.Println("categorization finished, update the block number to ", block_number, worker.smartcontract.NetworkId, worker.smartcontract.Address)
	worker.smartcontract.SetBlockParameter(block_number, block_timestamp)

	return block_number, nil
}
