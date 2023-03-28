package command

import (
	"github.com/blocklords/sds/categorizer/smartcontract"
)

type PushNewSmartcontracts struct {
	Smartcontracts []smartcontract.Smartcontract `json:"smartcontracts"`
}
