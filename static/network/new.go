package network

import (
	"errors"

	"github.com/blocklords/gosds/common/data_type/key_value"
)

// parses JSON object into the Network Type
func New(raw key_value.KeyValue) (*Network, error) {
	id, err := raw.GetString("id")
	if err != nil {
		return nil, err
	}

	flag_64, err := raw.GetUint64("flag")
	if err != nil {
		return nil, err
	}
	flag := int8(flag_64)
	if !IsValidFlag(flag) || flag == ALL {
		return nil, errors.New("invalid 'flag' from the parsed data")
	}

	provider, err := raw.GetString("provider")
	if err != nil {
		return nil, err
	}

	return &Network{
		Id:       id,
		Provider: provider,
		Flag:     flag,
	}, nil
}
