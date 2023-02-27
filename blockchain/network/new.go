package network

import (
	"github.com/blocklords/gosds/common/data_type/key_value"
	"github.com/blocklords/gosds/blockchain/network/provider"
)

// parses JSON object into the Network Type
func New(raw key_value.KeyValue) (*Network, error) {
	id, err := raw.GetString("id")
	if err != nil {
		return nil, err
	}

	raw_network_type, err := raw.GetString("type")
	if err != nil {
		return nil, err
	}

	network_type, err := NewNetworkType(raw_network_type)
	if err != nil {
		return nil, err
	}

	raw_providers, err := raw.GetKeyValueList("providers")
	if err != nil {
		return nil, err
	}
	providers, err := provider.NewList(raw_providers)
	if err != nil {
		return nil, err
	}

	return &Network{
		Id:        id,
		Providers: providers,
		Type:      network_type,
	}, nil
}
