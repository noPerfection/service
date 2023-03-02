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
	"github.com/charmbracelet/log"

	"github.com/blocklords/gosds/blockchain/transaction"

	"github.com/blocklords/gosds/blockchain/network"

	"github.com/blocklords/gosds/app/configuration"
	"github.com/blocklords/gosds/app/service"

	"github.com/blocklords/gosds/app/account"
	"github.com/blocklords/gosds/app/broadcast"
	"github.com/blocklords/gosds/app/controller"
	"github.com/blocklords/gosds/app/remote"
	"github.com/blocklords/gosds/app/remote/message"
	"github.com/blocklords/gosds/common/data_type/key_value"
	"github.com/blocklords/gosds/db"
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

	spaghetti_env, err := service.New(service.SPAGHETTI, service.BROADCAST, service.THIS)
	if err != nil {
		logger.Fatal("spaghetti service configuration", "message", err)
	}

	// we whitelist before we initiate the reply controller
	if !app_config.Plain {
		logger.Info("get the whitelisted services")

		whitelisted_services, err := get_whitelisted_services()
		if err != nil {
			panic(err)
		}
		accounts := account.NewServices(whitelisted_services)
		controller.AddWhitelistedAccounts(spaghetti_env, accounts)

		logger.Info("get the whitelisted subscribers")

		whitelisted_subscribers, err := get_whitelisted_subscribers()
		if err != nil {
			panic(err)
		}
		subsribers := account.NewServices(whitelisted_subscribers)

		broadcast.AddWhitelistedAccounts(spaghetti_env, subsribers)
	}

	reply, err := controller.NewReply(spaghetti_env)
	if err != nil {
		logger.Fatal("controller new", "message", err)
	} else {
		reply.SetLogger(logger)
	}

	broadcaster, err := broadcast.New(spaghetti_env, logger)
	if err != nil {
		logger.Fatal("broadcast", "message", err)
	}

	if !app_config.Plain {
		logger.Info("set the private keys")

		err := reply.SetControllerPrivateKey()
		if err != nil {
			logger.Fatal("controller.SetControllerPrivateKey", "message", err)
		}

		err = broadcaster.SetPrivateKey()
		if err != nil {
			logger.Fatal("broadcast.SetPrivateKey", "message", err)
		}
	}

	go broadcaster.Run()

	err = StartWorkers(logger, app_config)
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
