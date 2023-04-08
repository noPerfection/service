package network

import (
	"fmt"

	"github.com/blocklords/sds/blockchain/network/provider"
	"github.com/blocklords/sds/common/data_type/key_value"
)

// New Network from the key value object
func New(raw key_value.KeyValue) (*Network, error) {
	id, err := raw.GetString("id")
	if err != nil {
		return nil, fmt.Errorf("getting 'id' parameter '%v'; ", err)
	}

	raw_network_type, err := raw.GetString("type")
	if err != nil {
		return nil, fmt.Errorf("getting 'type' parameter '%v'; ", err)
	}

	network_type, err := NewNetworkType(raw_network_type)
	if err != nil {
		return nil, fmt.Errorf("converting 'type' parameter '%v'; ", err)
	}
	if network_type == ALL {
		return nil, fmt.Errorf("creating network with ALL is not valid")
	}

	raw_providers, err := raw.GetKeyValueList("providers")
	if err != nil {
		return nil, fmt.Errorf("getting 'providers' parameter '%v'; ", err)
	}
	providers, err := provider.NewList(raw_providers)
	if err != nil {
		return nil, fmt.Errorf("convertings 'provider' parameter '%v'; ", err)
	}
	if len(providers) == 0 {
		return nil, fmt.Errorf("atleast one provider should be given")
	}

	return &Network{
		Id:        id,
		Providers: providers,
		Type:      network_type,
	}, nil
}
