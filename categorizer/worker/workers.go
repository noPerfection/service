package worker

import (
	"fmt"
	"sort"

	"github.com/blocklords/gosds/db"
	"github.com/blocklords/gosds/message"
	static_abi "github.com/blocklords/gosds/static/abi"

	"github.com/blocklords/gosds/categorizer/abi"
	"github.com/blocklords/gosds/categorizer/smartcontract"
	"github.com/blocklords/gosds/remote"
)

type EvmWorkers []*Worker

// Creates the list of workers from the smartcontracts
func WorkersFromSmartcontracts(
	db *db.Database,
	static_socket *remote.Socket,
	smartcontracts []*smartcontract.Smartcontract,
	no_event bool,
	broadcast_channel chan message.Broadcast,
	in chan RequestSpaghettiBlockRange,
	out chan ReplySpaghettiBlockRange,
	log_in chan RequestLogParse,
	log_out chan ReplyLogParse,
) (EvmWorkers, error) {
	workers := make(EvmWorkers, 0)

	for _, smartcontract := range smartcontracts {
		if smartcontract.NetworkId == "imx" {
			continue
		}

		remote_abi, err := static_abi.Get(static_socket, smartcontract.NetworkId, smartcontract.Address)
		if err != nil {
			return nil, fmt.Errorf("failed to set the ABI from SDS Static. This is an exception. It should not happen. error: " + err.Error())
		}
		abi, err := abi.NewAbi(remote_abi)
		if err != nil {
			panic("failed to create a categorizer abi wrapper. error message: " + err.Error())
		}

		worker_smartcontract := smartcontract

		worker := NewWorker(db, abi, worker_smartcontract, no_event, broadcast_channel, in, out, log_in, log_out)

		workers = append(workers, worker)
	}

	return workers, nil
}

// Returns the list of workers where categorizated up until block_number
func (workers EvmWorkers) OldWorkers(block_number uint64) EvmWorkers {
	old_workers := make(EvmWorkers, 0)

	for _, worker := range workers {
		if worker.smartcontract.CategorizedBlockNumber < block_number {
			old_workers = append(old_workers, worker)
		}
	}

	return old_workers
}

// Returns the list of workers where categorizated up until block_number
func (workers EvmWorkers) RecentWorkers(block_number uint64) EvmWorkers {
	recent_workers := make(EvmWorkers, 0)

	for _, worker := range workers {
		if worker.smartcontract.CategorizedBlockNumber >= block_number {
			recent_workers = append(recent_workers, worker)
		}
	}

	return recent_workers
}

// Sort the workers from old to the newest
func (workers EvmWorkers) Sort() EvmWorkers {
	sort.SliceStable(workers, func(i, j int) bool {
		return workers[i].smartcontract.CategorizedBlockNumber < workers[j].smartcontract.CategorizedBlockNumber
	})

	return workers
}

// Returns the earliest block number
func (workers EvmWorkers) EarliestBlockNumber() uint64 {
	sorted_workers := workers.Sort()
	if len(sorted_workers) == 0 {
		return 0
	}

	return sorted_workers[0].smartcontract.CategorizedBlockNumber
}

func (workers EvmWorkers) RecentBlockNumber() uint64 {
	sorted_workers := workers.Sort()
	if len(sorted_workers) == 0 {
		return 0
	}

	latest := len(sorted_workers) - 1
	return sorted_workers[latest].smartcontract.CategorizedBlockNumber
}

// Returns the smartcontract information that should be categorized
func (workers EvmWorkers) GetSmartcontracts() []*smartcontract.Smartcontract {
	smartcontracts := make([]*smartcontract.Smartcontract, 0)

	for _, worker := range workers {
		smartcontracts = append(smartcontracts, worker.smartcontract)
	}

	return smartcontracts
}
