// Collection of workers as a data type
// And the functions that works with the workers collection
package evm

import (
	"sort"

	"github.com/blocklords/gosds/app/remote/message"
	"github.com/blocklords/gosds/db"

	"github.com/blocklords/gosds/app/remote"
	"github.com/blocklords/gosds/categorizer/smartcontract"
)

type EvmWorkers []*EvmWorker

// Creates the list of workers from the smartcontracts
func WorkersFromSmartcontracts(
	db *db.Database,
	static_socket *remote.Socket,
	smartcontracts []*smartcontract.Smartcontract,
	broadcast_channel chan message.Broadcast,
	log_in chan RequestLogParse,
	log_out chan ReplyLogParse,
) (EvmWorkers, error) {
	workers := make(EvmWorkers, 0)

	// for _, smartcontract := range smartcontracts {
	// if smartcontract.NetworkId == "imx" {
	// continue
	// }

	// remote_abi, err := static_abi.Get(static_socket, smartcontract.NetworkId, smartcontract.Address)
	// if err != nil {
	// return nil, fmt.Errorf("failed to set the ABI from SDS Static. This is an exception. It should not happen. error: " + err.Error())
	// }
	// abi, err := abi.NewAbi(remote_abi)
	// if err != nil {
	// panic("failed to create a categorizer abi wrapper. error message: " + err.Error())
	// }

	// worker_smartcontract := smartcontract

	// worker := New(db, abi, worker_smartcontract, broadcast_channel, in, out, log_in, log_out)

	// workers = append(workers, worker)
	// }

	return workers, nil
}

// Splits the workers to two workers by the block number
func (workers EvmWorkers) Split(block_number uint64) (EvmWorkers, EvmWorkers) {
	old_workers := make(EvmWorkers, 0)
	new_workers := make(EvmWorkers, 0)

	for _, worker := range workers {
		if worker.parent.Smartcontract.CategorizedBlockNumber < block_number {
			old_workers = append(old_workers, worker)
		} else {
			new_workers = append(new_workers, worker)
		}
	}

	return old_workers, new_workers
}

// Sort the workers from old to the newest
func (workers EvmWorkers) Sort() EvmWorkers {
	sort.SliceStable(workers, func(i, j int) bool {
		return workers[i].parent.Smartcontract.CategorizedBlockNumber < workers[j].parent.Smartcontract.CategorizedBlockNumber
	})

	return workers
}

// Returns the earliest block number
func (workers EvmWorkers) EarliestBlockNumber() uint64 {
	sorted_workers := workers.Sort()
	if len(sorted_workers) == 0 {
		return 0
	}

	return sorted_workers[0].parent.Smartcontract.CategorizedBlockNumber
}

func (workers EvmWorkers) RecentBlockNumber() uint64 {
	sorted_workers := workers.Sort()
	if len(sorted_workers) == 0 {
		return 0
	}

	latest := len(sorted_workers) - 1
	return sorted_workers[latest].parent.Smartcontract.CategorizedBlockNumber
}

// Returns the smartcontract information that should be categorized
func (workers EvmWorkers) GetSmartcontracts() []*smartcontract.Smartcontract {
	smartcontracts := make([]*smartcontract.Smartcontract, 0)

	for _, worker := range workers {
		smartcontracts = append(smartcontracts, worker.parent.Smartcontract)
	}

	return smartcontracts
}

// Returns the smartcontract address
func (workers EvmWorkers) GetSmartcontractAddresses() []string {
	addresses := make([]string, 0)

	for _, worker := range workers {
		addresses = append(addresses, worker.parent.Smartcontract.Address)
	}

	return addresses
}
