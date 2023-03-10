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

	"github.com/blocklords/sds/app/configuration"

	"github.com/blocklords/sds/app/remote"

	"github.com/blocklords/sds/app/remote/message"

	"github.com/blocklords/sds/app/service"

	"github.com/blocklords/sds/db"

	"github.com/blocklords/sds/app/controller"
)

var static_socket *remote.Socket

// Sends the smartcontracts to the blockchain package.
//
// The blockchain package will have the categorizer for its each blockchain type.
// They will handle the decoding the event logs.
// After decoding, the blockchain/categorizer will push back to this categorizer's puller.
func setup_smartcontracts(logger log.Logger, db_con *db.Database, network *network.Network) error {
	var mu sync.Mutex
	logger.Info("get all categorizable smartcontracts from database", "network_id", network.Id)
	smartcontracts, err := smartcontract.GetAllByNetworkId(db_con, network.Id)
	if err != nil {
		return fmt.Errorf("smartcontract.GetAllByNetworkId: %w", err)
	}

	logger.Info("all smartcontracts returned", "network_id", network.Id, "smartcontract amount", len(smartcontracts))

	logger.Info("send smartcontracts to blockchain/categorizer", "network_id", network.Id, "url", blockchain_proc.CategorizerManagerUrl(network.Id))

	request := message.Request{
		Command: "new-smartcontracts",
		Parameters: map[string]interface{}{
			"smartcontracts": smartcontracts,
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
		"smartcontract_set":     handler.SetSmartcontract,
		"snapshot_get":          handler.GetSnapshot,
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
