package network

import (
	"errors"

	"github.com/blocklords/gosds/app/remote"
	"github.com/blocklords/gosds/app/remote/message"
	"github.com/blocklords/gosds/common/data_type/key_value"
)

// Returns list of support network IDs from SDS Static
func GetRemoteNetworkIds(socket *remote.Socket, flag int8) ([]string, error) {
	if !IsValidFlag(flag) {
		return nil, errors.New("invalid 'flag' parameter")
	}
	request := message.Request{
		Command: "network_id_get_all",
		Parameters: map[string]interface{}{
			"flag": flag,
		},
	}

	params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return nil, err
	}
	return key_value.NewKeyValue(params).GetStringList("network_ids")
}

// Returns list of support network IDs from SDS Static
func GetRemoteNetworks(socket *remote.Socket, flag int8) (Networks, error) {
	if !IsValidFlag(flag) {
		return nil, errors.New("invalid 'flag' parameter")
	}
	request := message.Request{
		Command: "network_get_all",
		Parameters: map[string]interface{}{
			"flag": flag,
		},
	}

	params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return nil, err
	}
	raw_networks, err := key_value.NewKeyValue(params).GetMapList("networks")
	if err != nil {
		return nil, err
	}

	return NewNetworks(raw_networks)
}

// Returns the Blockchain Network access provider
func GetRemoteNetwork(socket *remote.Socket, network_id string, flag int8) (*Network, error) {
	if !IsValidFlag(flag) {
		return nil, errors.New("invalid 'flag' parameter")
	}
	request := message.Request{
		Command: "network_get",
		Parameters: map[string]interface{}{
			"network_id": network_id,
			"flag":       flag,
		},
	}

	raw_params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return nil, err
	}

	params := key_value.NewKeyValue(raw_params)

	raw, err := params.GetMap("network")
	if err != nil {
		return nil, err
	}

	return New(raw)
}
