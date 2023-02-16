package network

import (
	"errors"

	"github.com/blocklords/gosds/message"
)

// parses JSON object into the Network Type
func New(raw map[string]interface{}) (*Network, error) {
	id, err := message.GetString(raw, "id")
	if err != nil {
		return nil, err
	}

	flag_64, err := message.GetUint64(raw, "flag")
	if err != nil {
		return nil, err
	}
	flag := int8(flag_64)
	if !IsValidFlag(flag) || flag == ALL {
		return nil, errors.New("invalid 'flag' from the parsed data")
	}

	provider, err := message.GetString(raw, "provider")
	if err != nil {
		return nil, err
	}

	return &Network{
		Id:       id,
		Provider: provider,
		Flag:     flag,
	}, nil
}
