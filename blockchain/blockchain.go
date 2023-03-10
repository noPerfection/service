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
	app_log "github.com/blocklords/sds/app/log"
	blockchain_process "github.com/blocklords/sds/blockchain/inproc"
	"github.com/blocklords/sds/categorizer"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/charmbracelet/log"

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
func transaction_deployed_get(request message.Request, logger log.Logger, _parameters ...interface{}) message.Reply {
	network_id, err := request.Parameters.GetString("network_id")
	if err != nil {
		return message.Fail("validation: " + err.Error())
	}
	txid, err := request.Parameters.GetString("txid")
	if err != nil {
		return message.Fail("validation " + err.Error())
	}

	networks, err := network.GetNetworks(network.ALL)
	if err != nil {
		return message.Fail("network: " + err.Error())
	}

	if !networks.Exist(network_id) {
		return message.Fail("unsupported network id")
	}

	url := blockchain_process.BlockchainManagerUrl(network_id)
	sock := remote.InprocRequestSocket(url)
	defer sock.Close()

	tx_request := message.Request{
		Command: "transaction",
		Parameters: map[string]interface{}{
			"transaction_id": txid,
		},
	}

	blockchain_reply, err := sock.RequestRemoteService(&tx_request)
	if err != nil {
		return message.Fail("remote transaction_request: " + err.Error())
	}

	transaction_key_value, _ := blockchain_reply.GetKeyValue("transaction")

	reply := message.Reply{
		Status:     "OK",
		Message:    "",
		Parameters: transaction_key_value,
	}

	return reply
}

// Returns Network
func get_network(request message.Request, logger log.Logger, _parameters ...interface{}) message.Reply {
	command_logger := app_log.Child(logger, "network-get-command")
	command_logger.Info("incoming request", "parameters", request.Parameters)

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
func get_network_ids(request message.Request, logger log.Logger, _parameters ...interface{}) message.Reply {
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
func get_all_networks(request message.Request, logger log.Logger, _parameters ...interface{}) message.Reply {
	command_logger := app_log.Child(logger, "network-get-all-command")
	command_logger.Info("incoming request", "parameters", request.Parameters)

	raw_network_type, err := request.Parameters.GetString("network_type")
	if err != nil {
		return message.Fail("missing network_type parameter " + err.Error())
	}
	network_type, err := network.NewNetworkType(raw_network_type)
	if err != nil {
		return message.Fail("parameter 'network_type' has invalid type")
	}

	networks, err := network.GetNetworks(network_type)
	if err != nil {
		return message.Fail("blockchain " + err.Error())
	}

	return message.Reply{
		Status:     "OK",
		Message:    "",
		Parameters: key_value.Empty().Set("networks", networks),
	}
}

func Run(app_config *configuration.Config) {
	logger := app_log.New()
	logger.SetPrefix("blockchain")
	logger.SetReportCaller(true)
	logger.SetReportTimestamp(true)

	logger.Info("starting")

	spaghetti_env, err := service.Inprocess(service.SPAGHETTI)
	if err != nil {
		logger.Fatal("spaghetti service configuration", "message", err)
	}

	reply, err := controller.NewReply(spaghetti_env)
	if err != nil {
		logger.Fatal("controller new", "message", err)
	} else {
		reply.SetLogger(logger)
	}

	err = run_networks(logger, app_config)
	if err != nil {
		logger.Fatal("StartWorkers", "message", err)
	}

	var commands = controller.CommandHandlers{
		"transaction_deployed_get": transaction_deployed_get,
		"network_id_get_all":       get_network_ids,
		"network_get_all":          get_all_networks,
		"network_get":              get_network,
	}
	err = reply.Run(commands)
	if err != nil {
		logger.Fatal("controller error", "message", err)
	}
}

// Start the workers for each blockchain
func run_networks(logger log.Logger, app_config *configuration.Config) error {
	networks, err := network.GetNetworks(network.ALL)
	if err != nil {
		return fmt.Errorf("gosds/blockchain: failed to get networks: %v", err)
	}

	imx_network_found := false

	// if there are some logs, we should broadcast them to the SDS Categorizer
	pusher, err := categorizer.NewCategorizerPusher()
	if err != nil {
		logger.Fatal("create a pusher to SDS Categorizer", "message", err)
	}

	static_env, err := service.Inprocess(service.STATIC)
	if err != nil {
		logger.Fatal("new static service config", "message", err)
	}
	static_socket := remote.InprocRequestSocket(static_env.Url())

	for _, new_network := range networks {
		worker_logger := app_log.Child(logger, new_network.Type.String()+"_network_id_"+new_network.Id)
		worker_logger.SetReportCaller(false)

		if new_network.Type == network.EVM {
			blockchain_manager, err := evm_client.NewManager(new_network, worker_logger)
			if err != nil {
				return fmt.Errorf("gosds/blockchain: failed to create EVM client: %v", err)
			}
			go blockchain_manager.SetupSocket()

			// Categorizer of the smartcontracts
			// This categorizers are interacting with the SDS Categorizer
			categorizer := evm_categorizer.NewManager(worker_logger, new_network, pusher, static_socket)
			go categorizer.Start()
		} else if new_network.Type == network.IMX {
			imx_network_found = true

			new_client := imx_client.New(new_network)

			new_worker := imx_worker.New(app_config, new_client, worker_logger)
			go new_worker.SetupSocket()

			imx_manager := imx_categorizer.NewManager(app_config, new_network, pusher)
			go imx_manager.Start()
		} else {
			return fmt.Errorf("no blockchain handler for network_type %v", new_network.Type)
		}
	}

	if imx_network_found {
		err = imx.ValidateEnv(app_config)
		if err != nil {
			return fmt.Errorf("gosds/blockchain: failed to validate IMX specific config: %v", err)
		}
	}

	logger.Warn("all workers are running! Exit this goroutine")

	return nil
}
