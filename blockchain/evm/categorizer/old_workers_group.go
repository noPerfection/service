package categorizer

import "github.com/blocklords/gosds/blockchain/evm/categorizer/smartcontract"

// Worker Groups are list of smartcontracts
// That are categorized together.
type OldWorkerGroup struct {
	block_number uint64
	workers      smartcontract.EvmWorkers
}

type OldWorkerGroups []*OldWorkerGroup

// Create the old smartcontracts worker group
func NewGroup(block_number uint64, workers smartcontract.EvmWorkers) *OldWorkerGroup {
	return &OldWorkerGroup{
		block_number: block_number,
		workers:      workers,
	}
}

// Add new workers
func (group *OldWorkerGroup) add_workers(workers smartcontract.EvmWorkers) {
	group.workers = append(group.workers, workers...)
}

// Returns the categorizer group that is higher than the block_number
// That means, this categorizer group didn't reach to the block number.
//
// User can add his worker to this group. Then once its the time, categorization will happen.
func (groups OldWorkerGroups) FirstGroupGreaterThan(block_number uint64) *OldWorkerGroup {
	for _, group := range groups {
		if block_number > group.block_number {
			return group
		}
	}

	return nil
}

// Delete the group from the list of groups
// Manager has to reassign its old_categorizers to the updated version
func (groups OldWorkerGroups) Delete(group_to_delete *OldWorkerGroup) OldWorkerGroups {
	for i, group := range groups {
		if group == group_to_delete {
			return append(groups[:i], groups[i+1:]...)
		}
	}

	return nil
}
