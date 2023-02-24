// EVM blockchain worker's manager
// For every blockchain we have one manager.
// Manager keeps the list of the smartcontract workers:
// - list of workers for up to date smartcontracts
// - list of workers for categorization outdated smartcontracts
package worker

import (
	"github.com/blocklords/gosds/categorizer/smartcontract"
)

// Returns all smartcontracts from all managers
func GetSmartcontracts(managers map[string]*Manager) []*smartcontract.Smartcontract {
	smartcontracts := make([]*smartcontract.Smartcontract, 0)

	for _, manager := range managers {
		smartcontracts = append(smartcontracts, manager.GetSmartcontracts()...)
	}

	return smartcontracts
}
