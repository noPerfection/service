package handler

import (
	"github.com/blocklords/gosds/db"
	"github.com/blocklords/gosds/static/network"

	"github.com/blocklords/gosds/app/remote/message"
)

// Returns Network
func NetworkGet(_ *db.Database, request message.Request) message.Reply {
	network_id, err := message.GetString(request.Parameters, "network_id")
	if err != nil {
		return message.Fail(err.Error())
	}

	flag_64, err := message.GetUint64(request.Parameters, "flag")
	if err != nil {
		return message.Fail(err.Error())
	}
	flag := int8(flag_64)
	if !network.IsValidFlag(flag) {
		return message.Fail("'flag' parameter is invalid")
	}

	networks, err := network.GetNetworks(flag)
	if err != nil {
		return message.Fail(err.Error())
	}

	n, err := networks.Get(network_id)
	if err != nil {
		return message.Fail(err.Error())
	}

	reply := message.Reply{
		Status:  "OK",
		Message: "",
		Params: map[string]interface{}{
			"network": n.ToJSON(),
		},
	}

	return reply
}

// Returns an abi by the smartcontract key.
func NetworkGetIds(_ *db.Database, request message.Request) message.Reply {
	flag_64, err := message.GetUint64(request.Parameters, "flag")
	if err != nil {
		return message.Fail(err.Error())
	}
	flag := int8(flag_64)
	if !network.IsValidFlag(flag) {
		return message.Fail("'flag' parameter is invalid")
	}

	network_ids, err := network.GetNetworkIds(flag)
	if err != nil {
		return message.Fail(err.Error())
	}

	return message.Reply{
		Status:  "OK",
		Message: "",
		Params: map[string]interface{}{
			"network_ids": network_ids,
		},
	}
}

// Returns an abi by the smartcontract key.
func NetworkGetAll(_ *db.Database, request message.Request) message.Reply {
	flag_64, err := message.GetUint64(request.Parameters, "flag")
	if err != nil {
		return message.Fail(err.Error())
	}
	flag := int8(flag_64)
	if !network.IsValidFlag(flag) {
		return message.Fail("'flag' parameter is invalid")
	}

	networks, err := network.GetNetworks(flag)
	if err != nil {
		return message.Fail(err.Error())
	}

	raw_networks := make([]map[string]interface{}, len(networks))
	for i, n := range networks {
		raw_networks[i] = n.ToJSON()
	}

	return message.Reply{
		Status:  "OK",
		Message: "",
		Params: map[string]interface{}{
			"networks": raw_networks,
		},
	}
}
