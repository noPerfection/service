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
	app_log "github.com/blocklords/gosds/app/log"
	blockchain_process "github.com/blocklords/gosds/blockchain/inproc"
	"github.com/blocklords/gosds/categorizer"
	"github.com/charmbracelet/log"

	"github.com/blocklords/gosds/blockchain/transaction"

	"github.com/blocklords/gosds/blockchain/network"

	"github.com/blocklords/gosds/app/configuration"
	"github.com/blocklords/gosds/app/service"

	"github.com/blocklords/gosds/app/controller"
	"github.com/blocklords/gosds/app/remote"
	"github.com/blocklords/gosds/app/remote/message"
	"github.com/blocklords/gosds/common/data_type/key_value"
	"github.com/blocklords/gosds/db"

	"fmt"

	evm_categorizer "github.com/blocklords/gosds/blockchain/evm/categorizer"
	evm_log_parse "github.com/blocklords/gosds/blockchain/evm/categorizer/log_parse"
	imx_categorizer "github.com/blocklords/gosds/blockchain/imx/categorizer"

	evm_client "github.com/blocklords/gosds/blockchain/evm/client"
	imx_client "github.com/blocklords/gosds/blockchain/imx/client"

	"github.com/blocklords/gosds/blockchain/imx"
	imx_worker "github.com/blocklords/gosds/blockchain/imx/worker"
)

////////////////////////////////////////////////////////////////////
//
// Command handlers
//
////////////////////////////////////////////////////////////////////

// this function returns the smartcontract deployer, deployed block number
// and block timestamp by a transaction hash of the smartcontract deployment.
func transaction_deployed_get(_ *db.Database, request message.Request, logger log.Logger) message.Reply {
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

	tx_raw, _ := blockchain_reply.GetKeyValue("transaction")
	tx, _ := transaction.NewFromMap(tx_raw)

	reply := message.Reply{
		Status:  "OK",
		Message: "",
		Parameters: key_value.New(map[string]interface{}{
			"network_id":      network_id,
			"block_number":    tx.BlockNumber,
			"block_timestamp": tx.BlockTimestamp,
			"address":         tx.TxTo,
			"deployer":        tx.TxFrom,
			"txid":            txid,
		}),
	}

	return reply
}

func Run(app_config *configuration.Config) {
	logger := app_log.New()
	logger.SetPrefix("blockchain")
	logger.SetReportCaller(true)
	logger.SetReportTimestamp(true)

	logger.Info("starting")

	spaghetti_env, err := service.New(service.SPAGHETTI, service.THIS)
	if err != nil {
		logger.Fatal("spaghetti service configuration", "message", err)
	}

	// we whitelist before we initiate the reply controller
	if !app_config.Plain {
		whitelist_access(logger, spaghetti_env)
	}

	reply, err := controller.NewReply(spaghetti_env)
	if err != nil {
		logger.Fatal("controller new", "message", err)
	} else {
		reply.SetLogger(logger)
	}

	if !app_config.Plain {
		set_curve_key(logger, reply)
	}

	err = start_clients(logger, app_config)
	if err != nil {
		logger.Fatal("StartWorkers", "message", err)
	}

	var commands = controller.CommandHandlers{
		"transaction_deployed_get": transaction_deployed_get,
	}
	err = reply.Run(nil, commands)
	if err != nil {
		logger.Fatal("controller error", "message", err)
	}
}

// Start the workers
func start_clients(logger log.Logger, app_config *configuration.Config) error {
	networks, err := network.GetNetworks(network.ALL)
	if err != nil {
		return fmt.Errorf("gosds/blockchain: failed to get networks: %v", err)
	}

	evm_network_found := false
	imx_network_found := false

	// if there are some logs, we should broadcast them to the SDS Categorizer
	pusher, err := categorizer.NewCategorizerPusher()
	if err != nil {
		logger.Fatal("create a pusher to SDS Categorizer", "message", err)
	}

	for _, new_network := range networks {
		worker_logger := app_log.Child(logger, new_network.Type.String()+"_network_id_"+new_network.Id)
		worker_logger.SetReportCaller(false)

		if new_network.Type == network.EVM {
			evm_network_found = true

			blockchain_manager, err := evm_client.NewManager(new_network, worker_logger)
			if err != nil {
				return fmt.Errorf("gosds/blockchain: failed to create EVM client: %v", err)
			}
			go blockchain_manager.SetupSocket()

			// Categorizer of the smartcontracts
			// This categorizers are interacting with the SDS Categorizer
			categorizer := evm_categorizer.NewManager(worker_logger, new_network, pusher)
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

	if evm_network_found {
		go evm_log_parse.RunLogParse()
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
