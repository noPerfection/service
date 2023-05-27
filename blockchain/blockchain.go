// Package blockchain is the core service that acts as the gateway
// between other SDS services and blockchain network.
// All accesses to the blockchain network by SDS goes through blockchain service.
//
// Besides acting as the gateway, it also defines common blockchain data types:
//   - smartcontract events
//   - blockhain transaction
//
// Blockchain package also defines **network** sub package to handle the supported
// networks. Visit to [blockchain/network] for adding new supported networks.
//
// Each blockchain runs as a separate sds service.
//
// However their socket parameters are defined in [blockchain/inproc]
package blockchain

import (
	"github.com/blocklords/sds/blockchain/handler"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/service/communication/command"
	"github.com/blocklords/sds/service/log"
	"github.com/blocklords/sds/service/parameter"

	"github.com/blocklords/sds/blockchain/network"

	"github.com/blocklords/sds/service/configuration"

	"github.com/blocklords/sds/service/communication/message"
	"github.com/blocklords/sds/service/controller"
	"github.com/blocklords/sds/service/remote"
)

////////////////////////////////////////////////////////////////////
//
// Command handlers
//
////////////////////////////////////////////////////////////////////

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

	var request_parameters handler.GetNetworkRequest
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

	var request_parameters handler.GetNetworkIdsRequest
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

	reply := handler.GetNetworkIdsReply{
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

	var request_parameters handler.GetNetworksRequest
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

	reply := handler.GetNetworksReply{
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
		Add(handler.NETWORK_IDS_COMMAND, get_network_ids).
		Add(handler.NETWORKS_COMMAND, get_all_networks).
		Add(handler.NETWORK_COMMAND, get_network)
}

// Returns the parameter of the SDS Blockchain
func Service() *parameter.Service {
	service, _ := parameter.Inprocess(parameter.BLOCKCHAIN)
	return service
}

// Run the SDS Blockchain service.
// The SDS Blockchain will load the all supported networks from configuration.
//
// Then create the sub processes for each blockchain network.
//
// And finally enables the reply controller waiting for CommandHandlers
func Run(app_config *configuration.Config) {
	logger, _ := log.New("blockchain", log.WITH_TIMESTAMP)

	logger.Info("Setting default values for supported blockchain networks")
	app_config.SetDefault(network.SDS_BLOCKCHAIN_NETWORKS, network.DefaultConfiguration())

	this_service := Service()
	reply, err := controller.NewReply(this_service, logger)
	if err != nil {
		logger.Fatal("controller new", "message", err)
	}

	evm_service, err := parameter.NewExternal(parameter.EVM, parameter.REMOTE, app_config)
	if err != nil {
		logger.Fatal("parameter.NewExternal(parameter.EVM)", "error", err)
	}
	evm_socket, err := remote.NewTcpSocket(evm_service, &logger, app_config)
	if err != nil {
		logger.Fatal("remote.NewTcpSocket(EVM service)", "error", err)
	}

	imx_service, err := parameter.NewExternal(parameter.IMX, parameter.REMOTE, app_config)
	if err != nil {
		logger.Fatal("parameter.NewExternal(parameter.IMX)", "error", err)
	}
	imx_socket, err := remote.NewTcpSocket(imx_service, &logger, app_config)
	if err != nil {
		logger.Fatal("remote.NewTcpSocket(IMX service)", "error", err)
	}

	network_sockets := key_value.Empty().
		Set(network.EVM.String(), evm_socket).
		Set(network.IMX.String(), imx_socket)

	err = reply.Run(CommandHandlers(), app_config, network_sockets)
	if err != nil {
		logger.Fatal("controller error", "error", err)
	}
}
