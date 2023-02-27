package handler

import (
	"github.com/blocklords/gosds/blockchain/network"
	"github.com/blocklords/gosds/common/data_type/key_value"
	"github.com/blocklords/gosds/db"

	"github.com/blocklords/gosds/app/remote/message"
)

// Returns Network
func NetworkGet(_ *db.Database, request message.Request) message.Reply {
	network_id, err := request.Parameters.GetString("network_id")
	if err != nil {
		return message.Fail(err.Error())
	}

	raw_network_type, err := request.Parameters.GetString("network_type")
	if err != nil {
		return message.Fail(err.Error())
	}
	network_type, err := network.NewNetworkType(raw_network_type)
	if err != nil {
		return message.Fail("'network_type' parameter is invalid")
	}

	networks, err := network.GetNetworks(network_type)
	if err != nil {
		return message.Fail(err.Error())
	}

	n, err := networks.Get(network_id)
	if err != nil {
		return message.Fail(err.Error())
	}

	reply := message.Reply{
		Status:     "OK",
		Message:    "",
		Parameters: key_value.Empty().Set("network", n),
	}

	return reply
}

// Returns an abi by the smartcontract key.
func NetworkGetIds(_ *db.Database, request message.Request) message.Reply {
	raw_network_type, err := request.Parameters.GetString("network_type")
	if err != nil {
		return message.Fail(err.Error())
	}
	network_type, err := network.NewNetworkType(raw_network_type)
	if err != nil {
		return message.Fail("'network_type' parameter is invalid")
	}

	network_ids, err := network.GetNetworkIds(network_type)
	if err != nil {
		return message.Fail(err.Error())
	}

	return message.Reply{
		Status:  "OK",
		Message: "",
		Parameters: key_value.New(map[string]interface{}{
			"network_ids": network_ids,
		}),
	}
}

// Returns an abi by the smartcontract key.
func NetworkGetAll(_ *db.Database, request message.Request) message.Reply {
	raw_network_type, err := request.Parameters.GetString("network_type")
	if err != nil {
		return message.Fail(err.Error())
	}
	network_type, err := network.NewNetworkType(raw_network_type)
	if err != nil {
		return message.Fail("'network_type' parameter is invalid")
	}

	networks, err := network.GetNetworks(network_type)
	if err != nil {
		return message.Fail(err.Error())
	}

	return message.Reply{
		Status:     "OK",
		Message:    "",
		Parameters: key_value.Empty().Set("networks", networks),
	}
}
