package smartcontract

import "github.com/blocklords/gosds/message"

func New(blob map[string]interface{}) (*Smartcontract, error) {
	network_id, err := message.GetString(blob, "network_id")
	if err != nil {
		return nil, err
	}
	address, err := message.GetString(blob, "address")
	if err != nil {
		return nil, err
	}
	categorized_block_number, err := message.GetUint64(blob, "categorized_block_number")
	if err != nil {
		return nil, err
	}
	categorized_block_timestamp, err := message.GetUint64(blob, "categorized_block_timestamp")
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
