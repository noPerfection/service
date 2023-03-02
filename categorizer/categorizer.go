package categorizer

import (
	"fmt"

	app_log "github.com/blocklords/gosds/app/log"
	"github.com/charmbracelet/log"

	blockchain_proc "github.com/blocklords/gosds/blockchain/inproc"
	"github.com/blocklords/gosds/blockchain/network"
	"github.com/blocklords/gosds/categorizer/handler"
	"github.com/blocklords/gosds/categorizer/smartcontract"
	"github.com/blocklords/gosds/common/data_type/key_value"
	static_abi "github.com/blocklords/gosds/static/abi"

	"github.com/blocklords/gosds/app/account"
	"github.com/blocklords/gosds/app/configuration"

	"github.com/blocklords/gosds/app/remote"

	"github.com/blocklords/gosds/app/remote/message"

	"github.com/blocklords/gosds/app/service"

	"github.com/blocklords/gosds/db"

	"github.com/blocklords/gosds/app/controller"
)

var static_socket *remote.Socket

// Manages the EVM based smartcontracts on a certain blockchain
// todo use the blockchain/categorizer_push(network_id); defer close()
// then to categorizer_request.add_smartcontract(smartcontract)
func setup_smartcontracts(logger log.Logger, db_con *db.Database, network *network.Network) error {
	logger.Info("get all smartcontracts from database", "network_id", network.Id)
	smartcontracts, err := smartcontract.GetAllByNetworkId(db_con, network.Id)
	if err != nil {
		return fmt.Errorf("smartcontract.GetAllByNetworkId: %w", err)
	}

	logger.Info("all smartcontracts returned", "network_id", network.Id, "smartcontract amount", len(smartcontracts))

	pusher, err := blockchain_proc.CategorizerManagerSocket(network.Id)
	if err != nil {
		return fmt.Errorf("blockchain_proc.CategorizerManagerSocket %s network_id: %w", network.Id, err)
	}
	defer pusher.Close()

	static_abis := make([]*static_abi.Abi, len(smartcontracts))

	for i, smartcontract := range smartcontracts {
		logger.Info("get abi from static", "network_id", smartcontract.NetworkId, "address", smartcontract.Address)

		remote_abi, err := static_abi.Get(static_socket, smartcontract.NetworkId, smartcontract.Address)
		if err != nil {
			return fmt.Errorf("failed to set the ABI from SDS Static. This is an exception. It should not happen. error: " + err.Error())
		}

		static_abis[i] = remote_abi
	}

	logger.Info("send smartcontracts to blockchain/categorizer", "network_id", network.Id)

	request := message.Request{
		Command: "new-smartcontracts",
		Parameters: map[string]interface{}{
			"smartcontracts": smartcontracts,
			"abis":           static_abis,
		},
	}
	request_string, _ := request.ToString()

	_, err = pusher.SendMessage(request_string)
	if err != nil {
		return fmt.Errorf("failed to send: %w", err)
	}

	return nil
}

////////////////////////////////////////////////////////////////////
//
// Command handlers
//
////////////////////////////////////////////////////////////////////

// Saves the smartcontract in the database.
// then start a worker.
func smartcontract_set(db_con *db.Database, request message.Request, logger log.Logger) message.Reply {
	kv, err := request.Parameters.GetKeyValue("smartcontract")
	if err != nil {
		return message.Fail("missing 'smartcontract' parameter")
	}

	sm, err := smartcontract.New(kv)
	if err != nil {
		return message.Fail("request parameter -> smartcontract.New: " + err.Error())
	}

	if smartcontract.Exists(db_con, sm.NetworkId, sm.Address) {
		return message.Fail("the smartcontract already in SDS Categorizer")
	}

	saveErr := smartcontract.Save(db_con, sm)
	if saveErr != nil {
		return message.Fail("database: " + saveErr.Error())
	}

	pusher, err := blockchain_proc.CategorizerManagerSocket(sm.NetworkId)
	if err != nil {
		return message.Fail("inproc: " + err.Error())
	}
	defer pusher.Close()

	remote_abi, err := static_abi.Get(static_socket, sm.NetworkId, sm.Address)
	if err != nil {
		return message.Fail("failed to set the ABI from SDS Static. This is an exception. It should not happen. error: " + err.Error())
	}

	smartcontracts := []*smartcontract.Smartcontract{sm}
	static_abis := []*static_abi.Abi{remote_abi}

	push := message.Request{
		Command: "new-smartcontracts",
		Parameters: map[string]interface{}{
			"smartcontracts": smartcontracts,
			"abis":           static_abis,
		},
	}
	request_string, _ := push.ToString()

	_, err = pusher.SendMessage(request_string)
	if err != nil {
		return message.Fail("send: " + err.Error())
	}

	reply := message.Reply{
		Status:     "OK",
		Message:    "",
		Parameters: key_value.Empty().Set("smartcontract", sm),
	}

	return reply
}

// Smartcontract data are parsed and stored in the database
func Run(app_config *configuration.Config, db_con *db.Database) {
	logger := app_log.New()
	logger.SetPrefix("categorizer")
	logger.SetReportCaller(true)
	logger.SetReportTimestamp(true)

	logger.Info("starting")

	// check for missing environment variable otherwise exit.
	if _, err := service.New(service.SPAGHETTI, service.SUBSCRIBE, service.REMOTE); err != nil {
		logger.Fatal("new spaghetti service config", "message", err)
	}

	categorizer_env, err := service.New(service.CATEGORIZER, service.THIS)
	if err != nil {
		logger.Fatal("new categorizer service config", "message", err)
	}

	static_env, err := service.New(service.STATIC, service.REMOTE)
	if err != nil {
		logger.Fatal("new static service config", "message", err)
	}

	static_socket = remote.TcpRequestSocketOrPanic(static_env, categorizer_env)

	logger.Info("retreive networks", "network-type", network.ALL)

	networks, err := network.GetRemoteNetworks(static_socket, network.ALL)
	if err != nil {
		logger.Fatal("newwork.GetRemoteNetworks", "message", err)
	}

	logger.Info("networks retreived")

	for _, the_network := range networks {
		err := setup_smartcontracts(logger, db_con, the_network)
		if err != nil {
			logger.Fatal("setup_smartcontracts", "network_id", the_network.Id, "message", err)
		}
	}

	var commands = controller.CommandHandlers{
		"smartcontract_get_all": handler.GetSmartcontracts,
		"smartcontract_get":     handler.GetSmartcontract,

		"log_get_all": handler.GetLogs,

		"snapshot_get": handler.GetSnapshot,

		"smartcontract_set": smartcontract_set,
	}

	// we whitelist before we initiate the reply controller
	if !app_config.Plain {
		logger.Info("whitelisting accounts")
		whitelisted_services, err := get_whitelisted_services()
		if err != nil {
			panic(err)
		}
		accounts := account.NewServices(whitelisted_services)
		controller.AddWhitelistedAccounts(static_env, accounts)
	}

	reply, err := controller.NewReply(categorizer_env)
	if err != nil {
		logger.Fatal("controller.NewReply", "service", categorizer_env)
	} else {
		reply.SetLogger(logger)
	}

	if !app_config.Plain {
		logger.Info("set privatekey")
		err := reply.SetControllerPrivateKey()
		if err != nil {
			logger.Fatal("controller.SetControllerPrivateKey", "message", err)
		}
	}

	go SetupSocket(db_con)

	err = reply.Run(db_con, commands)
	if err != nil {
		logger.Fatal("controller.Run", "message", err)
	}
}
