package worker

type CategorizerGroup struct {
	block_number uint64
	workers      EvmWorkers
}

type CategorizerGroups []*CategorizerGroup

func NewCategorizerGroup(block_number uint64, workers EvmWorkers) *CategorizerGroup {
	return &CategorizerGroup{
		block_number: block_number,
		workers:      workers,
	}
}

func (group *CategorizerGroup) add_workers(workers EvmWorkers) {
	group.workers = append(group.workers, workers...)
}

// Returns the categorizer group that is higher than the block_number
// That means, this categorizer group didn't reach to the block number.
//
// User can add his worker to this group. Then once its the time, categorization will happen.
func (groups CategorizerGroups) GetUpcoming(block_number uint64) *CategorizerGroup {
	for _, group := range groups {
		if block_number > group.block_number {
			return group
		}
	}

	return nil
}

// Delete the group from the list of groups
// Manager has to reassign its old_categorizers to the updated version
func (groups CategorizerGroups) Delete(group_to_delete *CategorizerGroup) CategorizerGroups {
	for i, group := range groups {
		if group == group_to_delete {
			return append(groups[:i], groups[i+1:]...)
		}
	}

	return nil
}
