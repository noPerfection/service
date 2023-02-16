// The network package is used to get the blockchain network information.
// The storage.go file loads the network parameters from application environment.
//
// IMPORTANT! networks are not stored in the database! On environment variables only
package network

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/blocklords/gosds/env"
)

const (
	SUPPORTED_NETWORKS = "SUPPORTED_NETWORKS"
)

// Returns list of the blockchain networks
func GetNetworks(flag int8) (Networks, error) {
	if !IsValidFlag(flag) {
		return nil, errors.New("invalid 'flag' parameter value")
	}
	if !env.Exists(SUPPORTED_NETWORKS) {
		return nil, errors.New("the environment variable 'SUPPORTED_NETWORKS' is not provided")
	}

	env := env.GetString(SUPPORTED_NETWORKS)
	if len(env) == 0 {
		return nil, errors.New("the environment variable 'SUPPORTED_NETWORKS' is empty")
	}

	var raw_networks []map[string]interface{}
	decoder := json.NewDecoder(strings.NewReader(env))
	decoder.UseNumber()

	if err := decoder.Decode(&raw_networks); err != nil {
		return nil, err
	}

	networks := make([]*Network, 0)

	for _, raw := range raw_networks {
		network, err := New(raw)
		if err != nil {
			return nil, err
		}

		if flag == ALL || flag == network.Flag {
			networks = append(networks, network)
		}
	}

	return networks, nil
}

// Returns list of support network IDs
func GetNetworkIds(flag int8) ([]string, error) {
	networks, err := GetNetworks(flag)
	if err != nil {
		return nil, err
	}

	ids := make([]string, len(networks))

	if len(networks) == 0 {
		return ids, nil
	}

	for i, network := range networks {
		ids[i] = network.Id
	}
	return ids, nil
}
