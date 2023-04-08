// Package smartcontract binds the abi and smartcontract parameters.
// The aim of package is to decode the smartcontract event logs for the smartcontract
// using the abi.
package smartcontract

import (
	"fmt"

	"github.com/blocklords/sds/blockchain/evm/abi"
	"github.com/blocklords/sds/categorizer/event"
	categorizer_smartcontract "github.com/blocklords/sds/categorizer/smartcontract"

	spaghetti_log "github.com/blocklords/sds/blockchain/event"
)

// EvmWorker decodes the smartcontract event logs by abi.
type EvmWorker struct {
	abi *abi.Abi
	// todo remove from struct
	Smartcontract categorizer_smartcontract.Smartcontract
}

// New EvmWorker for the smartcontract.
func New(sm *categorizer_smartcontract.Smartcontract, abi *abi.Abi) *EvmWorker {
	return &EvmWorker{
		abi:           abi,
		Smartcontract: *sm,
	}
}

// DecodeLog categorizes raw event information based on the abi.
func (worker *EvmWorker) DecodeLog(raw_log *spaghetti_log.RawLog) (event.Log, error) {
	log_name, outputs, err := worker.abi.DecodeLog(raw_log.Topics, raw_log.Data)
	if err != nil {
		return event.Log{}, fmt.Errorf("abi.DecodeLog (event %d in transaction %s): %w", raw_log.Index, raw_log.Transaction.TransactionKey.Id, err)
	}

	l := event.New(log_name, outputs).AddMetadata(raw_log).AddSmartcontractData(&worker.Smartcontract)

	return *l, nil
}
