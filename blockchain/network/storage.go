// The network package is used to get the blockchain network information.
// The storage.go file loads the network parameters from application environment.
//
// IMPORTANT! networks are not stored in the database! On environment variables only
package network

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/blocklords/sds/app/configuration"
)

const (
	SDS_STATIC_NETWORKS = "SDS_STATIC_NETWORKS"
)

// Returns list of the blockchain networks
func GetNetworks(network_type NetworkType) (Networks, error) {
	network_config := configuration.New()
	network_config.SetDefault(SDS_STATIC_NETWORKS, DefaultConfiguration())

	env := network_config.GetString(SDS_STATIC_NETWORKS)
	if len(env) == 0 {
		return nil, errors.New("the environment variable 'SDS_STATIC_NETWORKS' is empty")
	}

	var raw_networks []map[string]interface{}
	decoder := json.NewDecoder(strings.NewReader(env))
	decoder.UseNumber()

	if err := decoder.Decode(&raw_networks); err != nil {
		return nil, errors.New("invalid json for SDS_STATIC_NETWORKS " + err.Error())
	}

	networks := make([]*Network, 0)

	for _, raw := range raw_networks {
		network, err := New(raw)
		if err != nil {
			return nil, errors.New("convert json to network " + err.Error())
		}

		if network_type == ALL || network_type == network.Type {
			networks = append(networks, network)
		}
	}

	return networks, nil
}

// Returns list of support network IDs
func GetNetworkIds(network_type NetworkType) ([]string, error) {
	networks, err := GetNetworks(network_type)
	if err != nil {
		return nil, fmt.Errorf("GetNetworks: %w", err)
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
