// Package smartcontract defines the smartcontract's categorization state.
//
// It's tracked on the database.
package smartcontract

import (
	"fmt"

	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/smartcontract_key"
)

// This data type is used to track categorization state per smartcontract
type Smartcontract struct {
	SmartcontractKey smartcontract_key.Key  `json:"smartcontract_key"`
	BlockHeader      blockchain.BlockHeader `json:"block_header"`
}

// Updates the categorized block parameter of the smartcontract.
// It means, this smartcontract 's' data was categorized till the given block numbers.
//
// The first is the block number, second is the block timestamp.
func (s *Smartcontract) SetBlockHeader(b blockchain.BlockHeader) {
	s.BlockHeader = b
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
