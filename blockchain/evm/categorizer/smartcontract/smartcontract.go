// EVM blockchain worker
package smartcontract

import (
	"fmt"

	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/blockchain/evm/abi"
	"github.com/blocklords/sds/blockchain/evm/categorizer/log_parse"
	"github.com/blocklords/sds/categorizer/event"
	categorizer_smartcontract "github.com/blocklords/sds/categorizer/smartcontract"

	spaghetti_log "github.com/blocklords/sds/blockchain/event"
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
func (worker *EvmWorker) DecodeLog(raw_log *spaghetti_log.Log) (*event.Log, error) {
	log_name, outputs, err := worker.abi.DecodeLog(raw_log.Topics, raw_log.Data)
	if err != nil {
		return nil, fmt.Errorf("abi.DecodeLog: %w", err)
	}

	l := event.New(log_name, outputs).AddMetadata(raw_log).AddSmartcontractData(worker.Smartcontract)

	return l, nil
}
