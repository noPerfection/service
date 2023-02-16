package smartcontract

import (
	"encoding/json"
)

type Smartcontract struct {
	NetworkId                 string
	Address                   string
	CategorizedBlockNumber    uint64
	CategorizedBlockTimestamp uint64
}

// Updates the categorized block parameter of the smartcontract.
// It means, this smartcontract 's' data was categorized till the given block numbers.
//
// The first is the block number, second is the block timestamp.
func (s *Smartcontract) SetBlockParameter(b uint64, t uint64) {
	s.CategorizedBlockNumber = b
	s.CategorizedBlockTimestamp = t
}

func (s *Smartcontract) ToJSON() map[string]interface{} {
	i := map[string]interface{}{}
	i["network_id"] = s.NetworkId
	i["address"] = s.Address
	i["categorized_block_number"] = s.CategorizedBlockNumber
	i["categorized_block_timestamp"] = s.CategorizedBlockTimestamp
	return i
}

// Returns a JSON representation of this smartcontract in a string format
func (b *Smartcontract) ToString() (string, error) {
	s := b.ToJSON()
	byt, err := json.Marshal(s)
	if err != nil {
		return "", err
	}

	return string(byt), nil
}
