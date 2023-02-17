package smartcontract

import (
	"github.com/blocklords/gosds/common/data_type/key_value"
)

func New(blob key_value.KeyValue) (*Smartcontract, error) {
	network_id, err := blob.GetString("network_id")
	if err != nil {
		return nil, err
	}
	address, err := blob.GetString("address")
	if err != nil {
		return nil, err
	}
	categorized_block_number, err := blob.GetUint64("categorized_block_number")
	if err != nil {
		return nil, err
	}
	categorized_block_timestamp, err := blob.GetUint64("categorized_block_timestamp")
	if err != nil {
		return nil, err
	}

	return &Smartcontract{
		NetworkId:                 network_id,
		Address:                   address,
		CategorizedBlockNumber:    categorized_block_number,
		CategorizedBlockTimestamp: categorized_block_timestamp,
	}, nil
}
