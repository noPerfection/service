package handler

import (
	"github.com/blocklords/sds/indexer/smartcontract"
)

// PushNewSmartcontracts defines the required
// parameters in message.Request.Parameters for
// NEW_CATEGORIZED_SMARTCONTRACTS
//
// Note that its for pull controllers,
// as a result it doesn't include the reply type.
type PushNewSmartcontracts struct {
	Smartcontracts []smartcontract.Smartcontract `json:"smartcontracts"`
}
