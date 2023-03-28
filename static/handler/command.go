package handler

import (
	"github.com/blocklords/sds/app/command"
)

const (
	// Direct
	GET_ABI command.Command = "abi_get"
	// Through the router
	SET_ABI command.Command = "abi_set"
	// Through the router
	GET_CONFIGURATION command.Command = "configuration_get"
	// Through the router
	SET_CONFIGURATION command.Command = "configuration_set"
	// Direct
	FILTER_SMARTCONTRACTS command.Command = "smartcontract_filter"
	// Through the router
	FILTER_SMARTCONTRACT_KEYS command.Command = "smartcontract_key_filter"
	// Through the router
	SET_SMARTCONTRACT command.Command = "smartcontract_set"
	// Direct
	GET_SMARTCONTRACT command.Command = "smartcontract_get"
)

// Return list of all commands and their handlers
func CommandHandlers() command.Handlers {
	return command.EmptyHandlers().
		Add(GET_ABI, AbiGetBySmartcontractKey).
		Add(SET_ABI, AbiRegister).
		Add(GET_SMARTCONTRACT, SmartcontractGet).
		Add(SET_SMARTCONTRACT, SmartcontractRegister).
		Add(FILTER_SMARTCONTRACTS, SmartcontractFilter).
		Add(FILTER_SMARTCONTRACT_KEYS, SmartcontractKeyFilter).
		Add(GET_CONFIGURATION, ConfigurationGet).
		Add(SET_CONFIGURATION, ConfigurationRegister)
}
