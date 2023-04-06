// The SDS Spaghetti module fetches the blockchain data and converts it into the internal format
// All other SDS Services are connecting to SDS Spaghetti.
//
// We have multiple workers.
// Atleast one worker for each network.
// This workers are called recent workers.
//
// Categorizer checks whether the cached block returned or not.
// If its a cached block, then switches to the block_range
package blockchain

import (
	"github.com/blocklords/sds/app/command"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/blockchain/handler"
	blockchain_process "github.com/blocklords/sds/blockchain/inproc"

	"github.com/blocklords/sds/blockchain/network"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/service"

	"github.com/blocklords/sds/app/controller"
	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/remote/message"

	"fmt"

	evm_categorizer "github.com/blocklords/sds/blockchain/evm/categorizer"
	imx_categorizer "github.com/blocklords/sds/blockchain/imx/categorizer"

	evm_client "github.com/blocklords/sds/blockchain/evm/client"
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
func transaction_deployed_get(request message.Request, logger log.Logger, parameters ...interface{}) message.Reply {
	if len(parameters) < 1 {
		return message.Fail("missing app configuration")
	}

	var request_parameters handler.DeployedTransactionRequest
	err := request.Parameters.ToInterface(&request_parameters)
	if err != nil {
		return message.Fail("failed to parse request parameters " + err.Error())
	}

	app_config, ok := parameters[0].(*configuration.Config)
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

	url := blockchain_process.ClientEndpoint(request_parameters.NetworkId)
	sock, err := remote.InprocRequestSocket(url, logger, app_config)
	if err != nil {
		return message.Fail("blockchain request error: " + err.Error())
	}
	defer sock.Close()

	req_parameters := handler.Transaction{
		TransactionId: request_parameters.TransactionId,
	}

	var blockchain_reply handler.LogFilterReply
	err = handler.DEPLOYED_TRANSACTION_COMMAND.Request(sock, req_parameters, &blockchain_reply)
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
	if len(app_parameters) < 1 {
		return message.Fail("missing app configuration")
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

	var reply handler.GetNetworkReply = *n
	reply_message, err := command.Reply(reply)
	if err != nil {
		return message.Fail("failed to reply: " + err.Error())
	}

	return reply_message
}

// Returns an abi by the smartcontract key.
func get_network_ids(request message.Request, _ log.Logger, app_parameters ...interface{}) message.Reply {
	if len(app_parameters) < 1 {
		return message.Fail("missing app configuration")
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

	var reply handler.GetNetworkIdsReply = network_ids
	reply_message, err := command.Reply(reply)
	if err != nil {
		return message.Fail("failed to reply: " + err.Error())
	}

	return reply_message
}

// Returns an abi by the smartcontract key.
func get_all_networks(request message.Request, logger log.Logger, app_parameters ...interface{}) message.Reply {
	if len(app_parameters) < 1 {
		return message.Fail("missing app configuration")
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

	var reply handler.GetNetworksReply = networks
	reply_message, err := command.Reply(reply)
	if err != nil {
		return message.Fail("failed to reply: " + err.Error())
	}

	return reply_message
}

// Return the list of command handlers for this service
func CommandHandlers() command.Handlers {
	return command.EmptyHandlers().
		Add(handler.DEPLOYED_TRANSACTION_COMMAND, transaction_deployed_get).
		Add(handler.NETWORK_IDS_COMMAND, get_network_ids).
		Add(handler.NETWORKS_COMMAND, get_all_networks).
		Add(handler.NETWORK_COMMAND, get_network)
}

// Returns this service's configuration
func Service() *service.Service {
	service, _ := service.Inprocess(service.SPAGHETTI)
	return service
}

// Start the blockchain service.
// The blockchain service has the three following parts:
//   - networks to load the network parameters from configuration
//   - blockchain client that connects SDS
//     to the remote blockchain node
//   - network worker for each network that consists the
//     blockchain client and atleast categorizer.
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

	err = reply.Run(CommandHandlers(), app_config)
	if err != nil {
		logger.Fatal("controller error", "message", err)
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

		if new_network.Type == network.EVM {
			blockchain_manager, err := evm_client.NewManager(new_network, worker_logger, app_config)
			if err != nil {
				return fmt.Errorf("gosds/blockchain: failed to create EVM client: %v", err)
			}
			go blockchain_manager.SetupSocket()

			// Categorizer of the smartcontracts
			// This categorizers are interacting with the SDS Categorizer
			categorizer, err := evm_categorizer.NewManager(worker_logger, new_network, app_config)
			if err != nil {
				worker_logger.Fatal("evm categorizer manager", "error", err)
			}
			go categorizer.Start()
		} else if new_network.Type == network.IMX {
			new_client := imx_client.New(new_network)

			new_worker := imx_worker.New(app_config, new_client, worker_logger)
			go new_worker.SetupSocket()

			imx_manager, err := imx_categorizer.NewManager(worker_logger, app_config, new_network)
			if err != nil {
				worker_logger.Fatal("imx.NewManager", "error", err)
			}
			go imx_manager.Start()
		} else {
			return fmt.Errorf("no blockchain handler for network_type %v", new_network.Type)
		}
	}

	return nil
}
