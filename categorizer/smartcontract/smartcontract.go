package smartcontract

import (
	"fmt"

	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/smartcontract_key"
)

type Smartcontract struct {
	Key   smartcontract_key.Key `json:"smartcontract_key"`
	Block blockchain.Block      `json:"block"`
}

// Updates the categorized block parameter of the smartcontract.
// It means, this smartcontract 's' data was categorized till the given block numbers.
//
// The first is the block number, second is the block timestamp.
func (s *Smartcontract) SetBlockParameter(b blockchain.Block) {
	s.Block = b
}

// Returns a JSON representation of this smartcontract in a string format
func (sm *Smartcontract) ToString() (string, error) {
	kv, err := key_value.NewFromInterface(sm)
	if err != nil {
		return "", fmt.Errorf("failed to serialize Smartcontract to intermediate key-value %v: %v", sm, err)
	}

	bytes, err := kv.ToBytes()
	if err != nil {
		return "", fmt.Errorf("failed to serialize intermediate key-value to string %v: %v", sm, err)
	}

	return string(bytes), nil
}
