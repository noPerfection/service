package network

import (
	"errors"

	"github.com/blocklords/gosds/message"
	"github.com/blocklords/gosds/remote"
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
	return message.GetStringList(params, "network_ids")
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
	raw_networks, err := message.GetMapList(params, "networks")
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

	params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return nil, err
	}
	raw, err := message.GetMap(params, "network")
	if err != nil {
		return nil, err
	}

	return New(raw)
}
