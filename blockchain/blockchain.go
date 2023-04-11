// Package blockchain defines core service that acts as the gateway
// between SDS and blockchain network.
// All accesses to the blockchain network within SDS goes through blockchain service.
//
// All blockchain specific reading/writing and categorizing the smartcontracts or any
// other feature are defined in this package as a sub package.
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

	"fmt"

	imx_categorizer "github.com/blocklords/sds/blockchain/imx/categorizer"
	imx_client "github.com/blocklords/sds/blockchain/imx/client"

	"github.com/blocklords/sds/blockchain/imx"
	imx_worker "github.com/blocklords/sds/blockchain/imx/worker"
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

	command_logger, err := logger.ChildWithoutReport("network-get-command")
	if err != nil {
		return message.Fail("network-get-command: " + err.Error())
	}
	command_logger.Info("incoming request", "parameters", request.Parameters)

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

	var network_type handler.GetNetworkIdsRequest
	err := request.Parameters.ToInterface(&network_type)
	if err != nil {
		return message.Fail("invalid parameters: " + err.Error())
	}

	app_config, ok := app_parameters[0].(*configuration.Config)
	if !ok {
		return message.Fail("the parameter is not app config")
	}
	network_ids, err := network.GetNetworkIds(app_config, network_type)
	if err != nil {
		return message.Fail(err.Error())
	}

	var reply = network_ids
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

	command_logger, err := logger.ChildWithoutReport("network-get-all-command")
	if err != nil {
		return message.Fail("network-get-all-command: " + err.Error())
	}
	command_logger.Info("incoming request", "parameters", request.Parameters)

	var network_type handler.GetNetworksRequest
	err = request.Parameters.ToInterface(&network_type)
	if err != nil {
		return message.Fail("invalid parameters: " + err.Error())
	}

	app_config, ok := app_parameters[0].(*configuration.Config)
	if !ok {
		return message.Fail("the parameter is not app config")
	}
	networks, err := network.GetNetworks(app_config, network_type)
	if err != nil {
		return message.Fail("blockchain " + err.Error())
	}

	var reply = networks
	reply_message, err := command.Reply(reply)
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

	this_service := Service()
	reply, err := controller.NewReply(this_service, logger)
	if err != nil {
		logger.Fatal("controller new", "message", err)
	}

	err = run_networks(logger, app_config)
	if err != nil {
		logger.Fatal("StartWorkers", "message", err)
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

// Start the workers for each blockchain network
// It will initiate the categorizer along with
// the client
func run_networks(logger log.Logger, app_config *configuration.Config) error {
	networks, err := network.GetNetworks(app_config, network.ALL)
	if err != nil {
		return fmt.Errorf("gosds/blockchain: failed to get networks: %v", err)
	}

	for _, new_network := range networks {
		if new_network.Type == network.IMX {
			err = imx.ValidateEnv(app_config)
			if err != nil {
				return fmt.Errorf("gosds/blockchain: failed to validate IMX specific config: %v", err)
			}
			break
		}
	}

	for _, new_network := range networks {
		worker_logger, err := logger.ChildWithTimestamp(new_network.Type.String() + "_network_id_" + new_network.Id)
		if err != nil {
			return fmt.Errorf("child logger: %w", err)
		}

		if new_network.Type == network.IMX {
			new_client := imx_client.New(new_network)

			new_worker := imx_worker.New(app_config, new_client, worker_logger)
			go new_worker.SetupSocket()

			imx_manager, err := imx_categorizer.NewManager(worker_logger, app_config, new_network)
			if err != nil {
				worker_logger.Fatal("imx.NewManager", "error", err)
			}
			go imx_manager.Start()
		} else if new_network.Type != network.EVM {
			return fmt.Errorf("no blockchain handler for network_type %v", new_network.Type)
		}
	}

	return nil
}
