package categorizer

import (
	"fmt"

	"github.com/blocklords/sds/app/log"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/controller"
	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/service"
	blockchain_command "github.com/blocklords/sds/blockchain/handler"
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
	logger.Info("get all categorizable smartcontracts from database", "network_id", network.Id)
	smartcontracts, err := smartcontract.GetAllByNetworkId(db_con, network.Id)
	if err != nil {
		return fmt.Errorf("smartcontract.GetAllByNetworkId: %w", err)
	}
	if len(smartcontracts) == 0 {
		return nil
	}

	logger.Info("all smartcontracts returned", "network_id", network.Id, "smartcontract amount", len(smartcontracts))
	logger.Info("send smartcontracts to blockchain/categorizer", "network_id", network.Id, "url", categorizer_process.CategorizerManagerUrl(network.Id))

	request := blockchain_command.PushNewSmartcontracts{
		Smartcontracts: smartcontracts,
	}
	err = blockchain_command.NEW_CATEGORIZED_SMARTCONTRACTS.Push(pusher, request)
	if err != nil {
		return fmt.Errorf("failed to send to blockchain package: %w", err)
	}

	return nil
}

// Return the list of command handlers for this service
var CommandHandlers = handler.CommandHandlers()

// Returns this service's configuration
// Could return nil, if the service is not found
func Service() *service.Service {
	service, _ := service.Inprocess(service.CATEGORIZER)
	return service
}

// This core service decodes the smartcontract event logs.
// The event data stored in the database.
// Provides commands to fetch the decoded logs from SDK.
//
// dep: SDS Blockchain core service
func Run(app_config *configuration.Config, db_con *db.Database) {
	logger, _ := log.New("categorizer", log.WITH_TIMESTAMP)

	logger.Info("starting")

	blockchain_service, err := service.Inprocess(service.SPAGHETTI)
	if err != nil {
		logger.Fatal("failed to get inproc configuration for the service", "service type", service.SPAGHETTI, "error", err)
	}

	blockchain_socket, err := remote.InprocRequestSocket(blockchain_service.Url(), logger, app_config)
	if err != nil {
		logger.Fatal("remote.InprocRequest", "url", blockchain_service.Url(), "error", err)
	}

	logger.Info("retreive networks", "network-type", network.ALL)

	request_parameters := blockchain_command.NetworkIds{
		NetworkType: network.ALL,
	}
	var parameters blockchain_command.NetworksReply
	err = blockchain_command.NETWORKS_COMMAND.Request(blockchain_socket, request_parameters, &parameters)
	blockchain_socket.Close()
	if err != nil {
		logger.Fatal("newwork.GetRemoteNetworks", "message", err)
	}

	logger.Info("networks retreived")

	pushers := make(map[string]*zmq.Socket, len(parameters.Networks))

	for _, the_network := range parameters.Networks {
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

	cat_service := Service()
	reply, err := controller.NewReply(cat_service, logger)
	if err != nil {
		logger.Fatal("controller.NewReply", "service", Service())
	}

	go RunPuller(logger, db_con)

	err = reply.Run(CommandHandlers, db_con, pushers)
	if err != nil {
		logger.Fatal("controller.Run", "error", err)
	}
}
