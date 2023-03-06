package network

import (
	"fmt"

	"github.com/blocklords/sds/common/data_type/key_value"
)

type Networks []*Network

// Whether the network with network_id exists in the networks list
func (networks Networks) Exist(network_id string) bool {
	for _, network := range networks {
		if network.Id == network_id {
			return true
		}
	}

	return false
}

// parses list of JSON objects into the list of Networks
func NewNetworks(raw_networks []key_value.KeyValue) (Networks, error) {
	networks := make(Networks, len(raw_networks))

	for i, raw := range raw_networks {
		network, err := New(raw)
		if err != nil {
			return nil, fmt.Errorf("raw_networks[%d] New: %w", i, err)
		}

		networks[i] = network
	}

	return networks, nil
}

// Returns the Network from the list of networks by its network_id
func (networks Networks) Get(network_id string) (*Network, error) {
	for _, network := range networks {
		if network.Id == network_id {
			return network, nil
		}
	}

	return nil, fmt.Errorf("'%s'not found", network_id)
}
