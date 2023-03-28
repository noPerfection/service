package handler

import (
	"github.com/blocklords/sds/app/remote/command"
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
