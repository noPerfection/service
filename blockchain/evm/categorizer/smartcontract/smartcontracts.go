// Collection of workers as a data type
// And the functions that works with the workers collection
package smartcontract

import (
	"sort"

	categorizer_smartcontract "github.com/blocklords/sds/categorizer/smartcontract"
)

type EvmWorkers []*EvmWorker

// Splits the workers to two workers by the block number
func (workers EvmWorkers) Split(block_number uint64) (EvmWorkers, EvmWorkers) {
	old_workers := make(EvmWorkers, 0)
	new_workers := make(EvmWorkers, 0)

	for _, worker := range workers {
		if worker.Smartcontract.BlockNumber < block_number {
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
		return workers[i].Smartcontract.BlockNumber < workers[j].Smartcontract.BlockNumber
	})

	return workers
}

// Returns the earliest block number
func (workers EvmWorkers) EarliestBlockNumber() uint64 {
	sorted_workers := workers.Sort()
	if len(sorted_workers) == 0 {
		return 0
	}

	return sorted_workers[0].Smartcontract.BlockNumber
}

func (workers EvmWorkers) RecentBlockNumber() uint64 {
	sorted_workers := workers.Sort()
	if len(sorted_workers) == 0 {
		return 0
	}

	latest := len(sorted_workers) - 1
	return sorted_workers[latest].Smartcontract.BlockNumber
}

// Returns the smartcontract information that should be categorized
func (workers EvmWorkers) GetSmartcontracts() []*categorizer_smartcontract.Smartcontract {
	smartcontracts := make([]*categorizer_smartcontract.Smartcontract, 0)

	for _, worker := range workers {
		smartcontracts = append(smartcontracts, worker.Smartcontract)
	}

	return smartcontracts
}

// Returns the smartcontract address
func (workers EvmWorkers) GetSmartcontractAddresses() []string {
	addresses := make([]string, 0)

	for _, worker := range workers {
		addresses = append(addresses, worker.Smartcontract.Address)
	}

	return addresses
}
