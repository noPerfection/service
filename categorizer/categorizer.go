package categorizer

import (
	"fmt"
	"sync"

	app_log "github.com/blocklords/sds/app/log"
	"github.com/charmbracelet/log"

	blockchain_proc "github.com/blocklords/sds/blockchain/inproc"
	"github.com/blocklords/sds/blockchain/network"
	"github.com/blocklords/sds/categorizer/handler"
	"github.com/blocklords/sds/categorizer/smartcontract"
	"github.com/blocklords/sds/common/data_type/key_value"
	static_abi "github.com/blocklords/sds/static/abi"

	"github.com/blocklords/sds/app/configuration"

	"github.com/blocklords/sds/app/remote"

	"github.com/blocklords/sds/app/remote/message"

	"github.com/blocklords/sds/app/service"

	"github.com/blocklords/sds/db"

	"github.com/blocklords/sds/app/controller"
)

var static_socket *remote.Socket

// Manages the EVM based smartcontracts on a certain blockchain
// todo use the blockchain/categorizer_push(network_id); defer close()
// then to categorizer_request.add_smartcontract(smartcontract)
func setup_smartcontracts(logger log.Logger, db_con *db.Database, network *network.Network) error {
	var mu sync.Mutex
	logger.Info("get all smartcontracts from database", "network_id", network.Id)
	smartcontracts, err := smartcontract.GetAllByNetworkId(db_con, network.Id)
	if err != nil {
		return fmt.Errorf("smartcontract.GetAllByNetworkId: %w", err)
	}

	logger.Info("all smartcontracts returned", "network_id", network.Id, "smartcontract amount", len(smartcontracts))

	static_abis := make([]*static_abi.Abi, len(smartcontracts))

	for i, smartcontract := range smartcontracts {
		logger.Info("get abi from static", "network_id", smartcontract.Key.NetworkId, "address", smartcontract.Key.Address)

		mu.Lock()
		remote_abi, err := static_abi.Get(static_socket, smartcontract.Key)
		mu.Unlock()
		if err != nil {
			return fmt.Errorf("failed to set the ABI from SDS Static. This is an exception. It should not happen. error: " + err.Error())
		}

		static_abis[i] = remote_abi
	}

	logger.Info("send smartcontracts to blockchain/categorizer", "network_id", network.Id, "url", blockchain_proc.CategorizerManagerUrl(network.Id))

	request := message.Request{
		Command: "new-smartcontracts",
		Parameters: map[string]interface{}{
			"smartcontracts": smartcontracts,
			"abis":           static_abis,
		},
	}
	request_string, _ := request.ToString()

	pusher, err := blockchain_proc.CategorizerManagerSocket(network.Id)
	if err != nil {
		return fmt.Errorf("blockchain_proc.CategorizerManagerSocket: %w", err)
	}
	defer pusher.Close()

	mu.Lock()
	_, err = pusher.SendMessage(request_string)
	mu.Unlock()
	if err != nil {
		return fmt.Errorf("failed to send to blockchain package: %w", err)
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
func smartcontract_set(request message.Request, logger log.Logger, parameters ...interface{}) message.Reply {
	db_con := parameters[0].(*db.Database)

	kv, err := request.Parameters.GetKeyValue("smartcontract")
	if err != nil {
		return message.Fail("missing 'smartcontract' parameter")
	}

	sm, err := smartcontract.New(kv)
	if err != nil {
		return message.Fail("request parameter -> smartcontract.New: " + err.Error())
	}

	if smartcontract.Exists(db_con, sm.Key) {
		return message.Fail("the smartcontract already in SDS Categorizer")
	}

	saveErr := smartcontract.Save(db_con, sm)
	if saveErr != nil {
		return message.Fail("database: " + saveErr.Error())
	}

	pusher, err := blockchain_proc.CategorizerManagerSocket(sm.Key.NetworkId)
	if err != nil {
		return message.Fail("inproc: " + err.Error())
	}
	defer pusher.Close()

	remote_abi, err := static_abi.Get(static_socket, sm.Key)
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

// This core service decodes the smartcontract event logs.
// The event data stored in the database.
// Provides commands to fetch the decoded logs from SDK.
//
// dep: SDS Static core service
func Run(app_config *configuration.Config, db_con *db.Database) {
	logger := app_log.New()
	logger.SetPrefix("categorizer")
	logger.SetReportCaller(true)
	logger.SetReportTimestamp(true)

	logger.Info("starting")

	categorizer_env, err := service.Inprocess(service.CATEGORIZER)
	if err != nil {
		logger.Fatal("new categorizer service config", "message", err)
	}

	static_env, err := service.Inprocess(service.STATIC)
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

		"snapshot_get": handler.GetSnapshot,

		"smartcontract_set": smartcontract_set,
	}

	reply, err := controller.NewReply(categorizer_env)
	if err != nil {
		logger.Fatal("controller.NewReply", "service", categorizer_env)
	} else {
		reply.SetLogger(logger)
	}

	go RunPuller(logger, db_con)

	err = reply.Run(commands, db_con)
	if err != nil {
		logger.Fatal("controller.Run", "error", err)
	}
}
