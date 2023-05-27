// Package handler defines the commands and command handlers
// that storage service's reply controller supports
package handler

import (
	"github.com/blocklords/sds/service/communication/command"
)

const (
	// Direct
	GET_ABI command.CommandName = "abi_get"
	// Through the router
	SET_ABI command.CommandName = "abi_set"
	// Through the router
	GET_CONFIGURATION command.CommandName = "configuration_get"
	// Through the router
	SET_CONFIGURATION command.CommandName = "configuration_set"
	// Direct
	FILTER_SMARTCONTRACTS command.CommandName = "smartcontract_filter"
	// Through the router
	FILTER_SMARTCONTRACT_KEYS command.CommandName = "smartcontract_key_filter"
	// Through the router
	SET_SMARTCONTRACT command.CommandName = "smartcontract_set"
	// Direct
	GET_SMARTCONTRACT command.CommandName = "smartcontract_get"
)

// Return list of all commands and their handlers
func CommandHandlers() command.Handlers {
	return command.EmptyHandlers().
		Add(GET_ABI, AbiGet).
		Add(SET_ABI, AbiRegister).
		Add(GET_SMARTCONTRACT, SmartcontractGet).
		Add(SET_SMARTCONTRACT, SmartcontractRegister).
		Add(FILTER_SMARTCONTRACTS, SmartcontractFilter).
		Add(FILTER_SMARTCONTRACT_KEYS, SmartcontractKeyFilter).
		Add(GET_CONFIGURATION, ConfigurationGet).
		Add(SET_CONFIGURATION, ConfigurationRegister)
}
