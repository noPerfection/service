package network

import (
	"fmt"

	"github.com/blocklords/gosds/app/remote"
	"github.com/blocklords/gosds/app/remote/message"
	"github.com/blocklords/gosds/common/data_type/key_value"
)

// Returns list of support network IDs from SDS Static
func GetRemoteNetworkIds(socket *remote.Socket, network_type NetworkType) ([]string, error) {
	request := message.Request{
		Command: "network_id_get_all",
		Parameters: map[string]interface{}{
			"network_type": network_type,
		},
	}

	params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return nil, fmt.Errorf("socket.RequestRemoteService: %w", err)
	}
	ids, err := key_value.New(params).GetStringList("network_ids")
	if err != nil {
		return nil, fmt.Errorf("network_ids reply parameter to string list: %w", err)
	}

	return ids, nil
}

// Returns list of support network IDs from SDS Static
func GetRemoteNetworks(socket *remote.Socket, network_type NetworkType) (Networks, error) {
	request := message.Request{
		Command: "network_get_all",
		Parameters: map[string]interface{}{
			"network_type": network_type,
		},
	}

	params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return nil, fmt.Errorf("failed to return network list from static socket: %v", err)
	}

	raw_networks, err := key_value.New(params).GetKeyValueList("networks")
	if err != nil {
		return nil, fmt.Errorf("failed convert parameters to the key value list: %v", err)
	}

	networks, err := NewNetworks(raw_networks)
	if err != nil {
		return nil, fmt.Errorf("networks reply parameter to network list: %w", err)
	}

	return networks, nil
}

// Returns the Blockchain Network access provider
func GetRemoteNetwork(socket *remote.Socket, network_id string, network_type NetworkType) (*Network, error) {
	request := message.Request{
		Command: "network_get",
		Parameters: map[string]interface{}{
			"network_id":   network_id,
			"network_type": network_type,
		},
	}

	raw_params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return nil, fmt.Errorf("failed to return network from static socket: %v", err)
	}

	params := key_value.New(raw_params)

	raw, err := params.GetKeyValue("network")
	if err != nil {
		return nil, fmt.Errorf("GetKeyValue(`network`): %w", err)
	}

	network, err := New(raw)
	if err != nil {
		return nil, fmt.Errorf("network reply parameter to network data type: %w", err)
	}

	return network, nil
}
