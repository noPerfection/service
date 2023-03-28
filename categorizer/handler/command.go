package handler

import (
	"github.com/blocklords/sds/app/command"
)

const (
	// Through the router
	SNAPSHOT command.Command = "snapshot_get"
	// Direct
	GET_SMARTCONTRACTS command.Command = "smartcontract_get_all"
	// Direct
	GET_SMARTCONTRACT command.Command = "smartcontract_get"
	// Through the router
	SET_SMARTCONTRACT command.Command = "smartcontract_set"
	// Internal from SDS Blockchain service to SDS Categorizer
	// Indicates that the list of smartcontracts are categorized
	CATEGORIZATION command.Command = "categorize"
)

// Return the list of command handlers for this service
// For the controller
func CommandHandlers() command.Handlers {
	return command.EmptyHandlers().
		Add(GET_SMARTCONTRACTS, GetSmartcontracts).
		Add(GET_SMARTCONTRACT, GetSmartcontract).
		Add(SET_SMARTCONTRACT, SetSmartcontract).
		Add(SNAPSHOT, GetSnapshot)
}
