package categorizer

import (
	"fmt"
	"sync"

	app_log "github.com/blocklords/sds/app/log"
	"github.com/charmbracelet/log"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/controller"
	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/app/service"
	categorizer_process "github.com/blocklords/sds/blockchain/inproc"
	"github.com/blocklords/sds/blockchain/network"
	"github.com/blocklords/sds/categorizer/handler"
	"github.com/blocklords/sds/categorizer/smartcontract"
	"github.com/blocklords/sds/db"

	zmq "github.com/pebbe/zmq4"
)

// Sends the smartcontracts to the blockchain package.
//
// The blockchain package will have the categorizer for its each blockchain type.
// They will handle the decoding the event logs.
// After decoding, the blockchain/categorizer will push back to this categorizer's puller.
func setup_smartcontracts(logger log.Logger, db_con *db.Database, network *network.Network, pusher *zmq.Socket) error {
	var mu sync.Mutex
	logger.Info("get all categorizable smartcontracts from database", "network_id", network.Id)
	smartcontracts, err := smartcontract.GetAllByNetworkId(db_con, network.Id)
	if err != nil {
		return fmt.Errorf("smartcontract.GetAllByNetworkId: %w", err)
	}

	logger.Info("all smartcontracts returned", "network_id", network.Id, "smartcontract amount", len(smartcontracts))

	logger.Info("send smartcontracts to blockchain/categorizer", "network_id", network.Id, "url", categorizer_process.CategorizerManagerUrl(network.Id))

	request := message.Request{
		Command: "new-smartcontracts",
		Parameters: map[string]interface{}{
			"smartcontracts": smartcontracts,
		},
	}
	request_string, _ := request.ToString()

	mu.Lock()
	_, err = pusher.SendMessage(request_string)
	mu.Unlock()
	if err != nil {
		return fmt.Errorf("failed to send to blockchain package: %w", err)
	}

	return nil
}

// Return the list of command handlers for this service
func CommandHandlers() controller.CommandHandlers {
	var commands = controller.CommandHandlers{
		"smartcontract_get_all": handler.GetSmartcontracts,
		"smartcontract_get":     handler.GetSmartcontract,
		"smartcontract_set":     handler.SetSmartcontract,
		"snapshot_get":          handler.GetSnapshot,
	}

	return commands
}

// Returns this service's configuration
func Service() *service.Service {
	return service.Inprocess(service.CATEGORIZER)
}

// This core service decodes the smartcontract event logs.
// The event data stored in the database.
// Provides commands to fetch the decoded logs from SDK.
//
// dep: SDS Blockchain core service
func Run(app_config *configuration.Config, db_con *db.Database) {
	logger := app_log.New()
	logger.SetPrefix("categorizer")
	logger.SetReportCaller(true)
	logger.SetReportTimestamp(true)

	logger.Info("starting")

	blockchain_service := service.Inprocess(service.SPAGHETTI)

	blockchain_socket := remote.InprocRequestSocket(blockchain_service.Url())

	logger.Info("retreive networks", "network-type", network.ALL)

	networks, err := network.GetRemoteNetworks(blockchain_socket, network.ALL)
	blockchain_socket.Close()
	if err != nil {
		logger.Fatal("newwork.GetRemoteNetworks", "message", err)
	}

	logger.Info("networks retreived")

	pushers := make(map[string]*zmq.Socket, len(networks))

	for _, the_network := range networks {
		pusher, err := categorizer_process.CategorizerManagerSocket(the_network.Id)
		if err != nil {
			logger.Fatal("categorizer_process.CategorizerManagerSocket: %w", err)
		}
		pushers[the_network.Id] = pusher

		err = setup_smartcontracts(logger, db_con, the_network, pusher)
		if err != nil {
			logger.Fatal("setup_smartcontracts", "network_id", the_network.Id, "message", err)
		}
	}

	reply, err := controller.NewReply(Service())
	if err != nil {
		logger.Fatal("controller.NewReply", "service", Service())
	} else {
		reply.SetLogger(logger)
	}

	go RunPuller(logger, db_con)

	err = reply.Run(CommandHandlers(), db_con, pushers)
	if err != nil {
		logger.Fatal("controller.Run", "error", err)
	}
}
