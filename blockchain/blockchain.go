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
// Each blockchain runs as a separate sds service. However blockchain package
//
// However their socket parameters are defined in [blockchain/inproc]
package blockchain

import (
	"github.com/blocklords/sds/app/command"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/blockchain/handler"
	blockchain_process "github.com/blocklords/sds/blockchain/inproc"
	"github.com/blocklords/sds/common/data_type/key_value"

	"github.com/blocklords/sds/blockchain/network"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/service"

	"github.com/blocklords/sds/app/controller"
	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/remote/message"
)

////////////////////////////////////////////////////////////////////
//
// Command handlers
//
////////////////////////////////////////////////////////////////////

// this function returns the smartcontract deployer, deployed block number
// and block timestamp by a transaction hash of the smartcontract deployment.
func transaction_deployed_get(request message.Request, logger log.Logger, app_parameters ...interface{}) message.Reply {
	if len(app_parameters) < 2 {
		return message.Fail("missing app configuration and network sockets")
	}

	var request_parameters handler.DeployedTransactionRequest
	err := request.Parameters.ToInterface(&request_parameters)
	if err != nil {
		return message.Fail("failed to parse request parameters " + err.Error())
	}

	app_config, ok := app_parameters[0].(*configuration.Config)
	if !ok {
		return message.Fail("the parameter is not app config")
	}
	networks, err := network.GetNetworks(app_config, network.ALL)
	if err != nil {
		return message.Fail("network: " + err.Error())
	}

	if !networks.Exist(request_parameters.NetworkId) {
		return message.Fail("unsupported network id")
	}

	network, err := networks.Get(request_parameters.NetworkId)
	if err != nil {
		return message.Fail("network.Get: " + err.Error())
	}

	network_sockets := app_parameters[1].(key_value.KeyValue)
	sock, ok := network_sockets[network.Type.String()].(*remote.ClientSocket)
	if !ok {
		return message.Fail("no network client socket was registered of for " + network.Type.String() + " network type")
	}

	url := blockchain_process.ClientEndpoint(request_parameters.NetworkId)
	target_service, err := service.InprocessFromUrl(url)
	if err != nil {
		return message.Fail("service.InprocessFromUrl(url): " + err.Error())
	}

	req_parameters := handler.DeployedTransactionRequest{
		NetworkId:     network.Id,
		TransactionId: request_parameters.TransactionId,
	}
	var blockchain_reply handler.DeployedTransactionReply
	err = handler.DEPLOYED_TRANSACTION_COMMAND.RequestRouter(sock, target_service, req_parameters, &blockchain_reply)
	if err != nil {
		return message.Fail("remote transaction_request: " + err.Error())
	}

	reply, err := command.Reply(blockchain_reply)
	if err != nil {
		return message.Fail("reply preparation: " + err.Error())
	}

	return reply
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

// CommandHandlers returns the list of commands and their handlers for SDS Spaghetti reply
// contorller. That means it will expose the following commands.
//
// SDS Spaghetti defines has the following commands:
//
//   - handler.DEPLOYED_TRANSACTION_COMMAND
//   - handler.NETWORK_IDS_COMMAND
//   - handler.NETWORKS_COMMAND
//   - handler NETWORK_COMMAND
//
// Check out "handler" sub package for the description of each command.
func CommandHandlers() command.Handlers {
	return command.EmptyHandlers().
		Add(handler.DEPLOYED_TRANSACTION_COMMAND, transaction_deployed_get).
		Add(handler.NETWORK_IDS_COMMAND, get_network_ids).
		Add(handler.NETWORKS_COMMAND, get_all_networks).
		Add(handler.NETWORK_COMMAND, get_network)
}

// Returns the parameter of the SDS Spaghetti
func Service() *service.Service {
	service, _ := service.Inprocess(service.SPAGHETTI)
	return service
}

// Run the SDS Spaghetti service.
// The SDS Spaghetti will load the all supported networks from configuration.
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

	evm_service, err := service.NewExternal(service.EVM, service.REMOTE, app_config)
	if err != nil {
		logger.Fatal("service.NewExternal(service.EVM)", "error", err)
	}
	evm_socket, err := remote.NewTcpSocket(evm_service, logger, app_config)
	if err != nil {
		logger.Fatal("remote.NewTcpSocket(EVM service)", "error", err)
	}

	imx_service, err := service.NewExternal(service.IMX, service.REMOTE, app_config)
	if err != nil {
		logger.Fatal("service.NewExternal(service.IMX)", "error", err)
	}
	imx_socket, err := remote.NewTcpSocket(imx_service, logger, app_config)
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
