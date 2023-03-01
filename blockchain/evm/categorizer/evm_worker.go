// EVM blockchain worker
package categorizer

import (
	"fmt"
	"sync"

	"github.com/blocklords/gosds/app/remote"
	"github.com/blocklords/gosds/blockchain/evm/abi"
	"github.com/blocklords/gosds/categorizer/event"
	"github.com/blocklords/gosds/categorizer/smartcontract"

	spaghetti_log "github.com/blocklords/gosds/blockchain/event"
)

// For EVM based smartcontracts
type EvmWorker struct {
	abi           *abi.Abi
	log_sock      *remote.Socket
	smartcontract *smartcontract.Smartcontract
}

// Wraps the Worker with the EVM related data and returns the wrapped Worker as EvmWorker
func New(sm *smartcontract.Smartcontract, abi *abi.Abi) *EvmWorker {
	log_sock := remote.InprocRequestSocket(LOG_PARSE_URL)

	return &EvmWorker{
		abi:           abi,
		smartcontract: sm,
		log_sock:      log_sock,
	}
}

// Categorize the blocks for this smartcontract
func (worker *EvmWorker) categorize(logs []*spaghetti_log.Log) ([]*event.Log, uint64) {
	var mu sync.Mutex
	network_id := worker.smartcontract.NetworkId
	address := worker.smartcontract.Address

	var block_number uint64 = worker.smartcontract.CategorizedBlockNumber
	var block_timestamp uint64 = worker.smartcontract.CategorizedBlockTimestamp

	categorized_logs := make([]*event.Log, 0, len(logs))

	if len(logs) > 0 {
		for log_index := 0; log_index < len(logs); log_index++ {
			raw_log := logs[log_index]

			mu.Lock()
			fmt.Println("requesting parse of smartcontract log to SDS Log...", raw_log, worker.smartcontract)
			log_name, outputs, err := ParseLog(worker.log_sock, network_id, address, raw_log.Data, raw_log.Topics)
			mu.Unlock()
			fmt.Println("reply received from SDS Log")
			if err != nil {
				fmt.Println("abi.remote parse %w, we skip this log records", err)
				continue
			}

			l := event.New(log_name, outputs).AddMetadata(raw_log).AddSmartcontractData(worker.smartcontract)

			if l.BlockNumber > block_number {
				block_number = l.BlockNumber
				block_timestamp = l.BlockTimestamp
			}

			categorized_logs = append(categorized_logs, l)
		}
	}

	fmt.Println("categorization finished, update the block number to ", block_number, worker.smartcontract.NetworkId, worker.smartcontract.Address)
	worker.smartcontract.SetBlockParameter(block_number, block_timestamp)

	return categorized_logs, block_number
}
