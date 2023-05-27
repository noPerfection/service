package handler

import (
	"github.com/blocklords/sds/blockchain/network"
	"github.com/blocklords/sds/service/communication/command"
	"github.com/blocklords/sds/service/communication/message"
	"github.com/blocklords/sds/service/configuration"
	"github.com/blocklords/sds/service/log"
)

// GetNetworkRequest defines the required
// parameters in message.Request.Parameters for
// NETWORK_COMMAND
type GetNetworkRequest struct {
	NetworkId   string              `json:"network_id"`
	NetworkType network.NetworkType `json:"network_type"`
}

// GetNetworkReply defines keys and value types
// of message.Reply.Parameters that are returned
// by controller after handling NETWORK_COMMAND
type GetNetworkReply = network.Network

// GetNetworksRequest defines the required
// parameters in message.Request.Parameters for
// NETWORKS_COMMAND
type GetNetworksRequest struct {
	NetworkType network.NetworkType
}

// GetNetworksReply defines keys and value types
// of message.Reply.Parameters that are returned
// by controller after handling NETWORKS_COMMAND
type GetNetworksReply struct {
	Networks network.Networks `json:"networks"`
}

// GetNetworkIdsReply defines the required
// parameters in message.Request.Parameters for
// NETWORK_IDS_COMMAND
type GetNetworkIdsRequest struct {
	NetworkType network.NetworkType
}

// GetNetworksReply defines keys and value types
// of message.Reply.Parameters that are returned
// by controller after handling NETWORK_IDS_COMMAND
type GetNetworkIdsReply struct {
	NetworkIds []string `json:"network_ids"`
}

// Returns Network
func get_network(request message.Request, logger log.Logger, app_parameters ...interface{}) message.Reply {
	if len(app_parameters) < 2 {
		return message.Fail("missing app configuration and network sockets")
	}

	command_logger, err := logger.Child("handler", "command", request.Command)
	if err != nil {
		return message.Fail("network-get-command: " + err.Error())
	}
	command_logger.Info("Get network", "parameters", request.Parameters)

	var request_parameters GetNetworkRequest
	err = request.Parameters.ToInterface(&request_parameters)
	if err != nil {
		return message.Fail("failed to parse request parameters " + err.Error())
	}

	app_config, ok := app_parameters[0].(*configuration.Config)
	if !ok {
		return message.Fail("the parameter is not app config")
	}
	networks, err := network.GetNetworks(app_config, request_parameters.NetworkType)
	if err != nil {
		return message.Fail(err.Error())
	}

	n, err := networks.Get(request_parameters.NetworkId)
	if err != nil {
		return message.Fail(err.Error())
	}

	var reply = *n
	reply_message, err := command.Reply(reply)
	if err != nil {
		return message.Fail("failed to reply: " + err.Error())
	}

	return reply_message
}

// Returns an abi by the smartcontract key.
func get_network_ids(request message.Request, _ log.Logger, app_parameters ...interface{}) message.Reply {
	if len(app_parameters) < 2 {
		return message.Fail("missing app configuration and network sockets")
	}

	var request_parameters GetNetworkIdsRequest
	err := request.Parameters.ToInterface(&request_parameters)
	if err != nil {
		return message.Fail("invalid parameters: " + err.Error())
	}

	app_config, ok := app_parameters[0].(*configuration.Config)
	if !ok {
		return message.Fail("the parameter is not app config")
	}
	network_ids, err := network.GetNetworkIds(app_config, request_parameters.NetworkType)
	if err != nil {
		return message.Fail(err.Error())
	}

	reply := GetNetworkIdsReply{
		NetworkIds: network_ids,
	}
	reply_message, err := command.Reply(reply)
	if err != nil {
		return message.Fail("failed to reply: " + err.Error())
	}

	return reply_message
}

// Returns an abi by the smartcontract key.
func get_all_networks(request message.Request, logger log.Logger, app_parameters ...interface{}) message.Reply {
	if len(app_parameters) < 2 {
		return message.Fail("missing app configuration and network sockets")
	}

	command_logger, err := logger.Child("handler", "command", request.Command)
	if err != nil {
		return message.Fail("network-get-all-command: " + err.Error())
	}
	command_logger.Info("Get all networks", "parameters", request.Parameters)

	var request_parameters GetNetworksRequest
	err = request.Parameters.ToInterface(&request_parameters)
	if err != nil {
		return message.Fail("invalid parameters: " + err.Error())
	}

	app_config, ok := app_parameters[0].(*configuration.Config)
	if !ok {
		return message.Fail("the parameter is not app config")
	}
	networks, err := network.GetNetworks(app_config, request_parameters.NetworkType)
	if err != nil {
		return message.Fail("blockchain " + err.Error())
	}

	reply := GetNetworksReply{
		Networks: networks,
	}
	reply_message, err := command.Reply(&reply)
	if err != nil {
		return message.Fail("failed to reply: " + err.Error())
	}

	return reply_message
}

// CommandHandlers returns the list of commands and their handlers for SDS Blockchain reply
// contorller. That means it will expose the following commands.
//
// SDS Blockchain defines has the following commands:
//
//   - handler.DEPLOYED_TRANSACTION_COMMAND
//   - handler.NETWORK_IDS_COMMAND
//   - handler.NETWORKS_COMMAND
//   - handler NETWORK_COMMAND
//
// Check out "handler" sub package for the description of each command.
func CommandHandlers() command.Handlers {
	return command.EmptyHandlers().
		Add(NETWORK_IDS_COMMAND, get_network_ids).
		Add(NETWORKS_COMMAND, get_all_networks).
		Add(NETWORK_COMMAND, get_network)
}
