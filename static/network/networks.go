package network

import "errors"

type Networks []*Network

// parses list of JSON objects into the list of Networks
func NewNetworks(raw_networks []map[string]interface{}) (Networks, error) {
	networks := make(Networks, len(raw_networks))

	for i, raw := range raw_networks {
		network, err := New(raw)
		if err != nil {
			return nil, err
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

	return nil, errors.New("network not found")
}
