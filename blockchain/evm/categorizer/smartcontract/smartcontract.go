// EVM blockchain worker
package smartcontract

import (
	"fmt"
	"sync"

	"github.com/blocklords/gosds/app/remote"
	"github.com/blocklords/gosds/blockchain/evm/abi"
	"github.com/blocklords/gosds/blockchain/evm/categorizer/log_parse"
	"github.com/blocklords/gosds/categorizer/event"
	categorizer_smartcontract "github.com/blocklords/gosds/categorizer/smartcontract"

	spaghetti_log "github.com/blocklords/gosds/blockchain/event"
)

// For EVM based smartcontracts
type EvmWorker struct {
	abi *abi.Abi
	// todo remove from struct
	log_sock      *remote.Socket
	Smartcontract *categorizer_smartcontract.Smartcontract
}

// Wraps the Worker with the EVM related data and returns the wrapped Worker as EvmWorker
func New(sm *categorizer_smartcontract.Smartcontract, abi *abi.Abi) *EvmWorker {
	log_sock := remote.InprocRequestSocket(log_parse.LOG_PARSE_URL)

	return &EvmWorker{
		abi:           abi,
		Smartcontract: sm,
		log_sock:      log_sock,
	}
}

// Categorize the blocks for this smartcontract
func (worker *EvmWorker) Categorize(logs []*spaghetti_log.Log) ([]*event.Log, uint64) {
	var mu sync.Mutex
	network_id := worker.Smartcontract.NetworkId
	address := worker.Smartcontract.Address
	block_number := worker.Smartcontract.CategorizedBlockNumber
	block_timestamp := worker.Smartcontract.CategorizedBlockTimestamp

	categorized_logs := make([]*event.Log, 0, len(logs))

	if len(logs) > 0 {
		for log_index := 0; log_index < len(logs); log_index++ {
			raw_log := logs[log_index]

			mu.Lock()
			fmt.Println("requesting parse of smartcontract log to SDS Log...", raw_log, worker.Smartcontract)
			log_name, outputs, err := log_parse.ParseLog(worker.log_sock, network_id, address, raw_log.Data, raw_log.Topics)
			mu.Unlock()
			fmt.Println("reply received from SDS Log")
			if err != nil {
				fmt.Println("abi.remote parse %w, we skip this log records", err)
				continue
			}

			l := event.New(log_name, outputs).AddMetadata(raw_log).AddSmartcontractData(worker.Smartcontract)

			if l.BlockNumber > block_number {
				block_number = l.BlockNumber
				block_timestamp = l.BlockTimestamp
			}

			categorized_logs = append(categorized_logs, l)
		}
	}

	fmt.Println("categorization finished, update the block number to ", block_number, worker.Smartcontract.NetworkId, worker.Smartcontract.Address)
	worker.Smartcontract.SetBlockParameter(block_number, block_timestamp)

	return categorized_logs, block_number
}
