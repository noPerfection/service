package network

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/blocklords/sds/service/configuration"
)

const (
	// Configuration name that handles the list of [network.Networks]
	SDS_BLOCKCHAIN_NETWORKS = "SDS_BLOCKCHAIN_NETWORKS"
)

// Returns list of the blockchain networks from configuration
func GetNetworks(network_config *configuration.Config, network_type NetworkType) (Networks, error) {
	if !network_config.Exist(SDS_BLOCKCHAIN_NETWORKS) {
		return nil, fmt.Errorf("missing '%s' in the configuration, atleast call config.SetDefault(network.SDS_BLOCKCHAIN_NETWORKS, network.DefaultConfiguration())", SDS_BLOCKCHAIN_NETWORKS)
	}
	if !network_type.valid() {
		return nil, fmt.Errorf("unsupported network type %s", network_type.String())
	}
	env := network_config.GetString(SDS_BLOCKCHAIN_NETWORKS)
	if len(env) == 0 {
		return nil, errors.New("the environment variable 'SDS_BLOCKCHAIN_NETWORKS' is empty")
	}

	var raw_networks []map[string]interface{}
	decoder := json.NewDecoder(strings.NewReader(env))
	decoder.UseNumber()

	if err := decoder.Decode(&raw_networks); err != nil {
		return nil, errors.New("invalid json for SDS_BLOCKCHAIN_NETWORKS " + err.Error())
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

// Returns list of support network IDs from configuration
func GetNetworkIds(network_config *configuration.Config, network_type NetworkType) ([]string, error) {
	networks, err := GetNetworks(network_config, network_type)
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
