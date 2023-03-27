package handler

import (
	"github.com/blocklords/sds/app/remote/command"
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
)
