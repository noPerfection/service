// Package categorizer defines the service that
// decodes the raw smartcontract events based on the abi.
//
// The decoded parameters are then saved in the database.
//
// This package defines the reply controller that allows external users
// to request decoded smartcontract event logs.
//
// Categorizer is part of SDS Core.
//
// Note that this package doesn't connect to the remote blockchain node to fetch the smartcontract logs.
//
// Rather it works with the blockchain/<network type>/categorizer sub processes.
// The categorizer sub processes in the the blockchain service will do all the work and
// notify this package with the ready to use event logs.
//
// This package will saves them in the database and allows users to fetch them.
package categorizer

import (
	"github.com/blocklords/sds/app/log"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/controller"
	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/service"
	blockchain_command "github.com/blocklords/sds/blockchain/handler"
	"github.com/blocklords/sds/blockchain/network"
	"github.com/blocklords/sds/categorizer/handler"
)

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
func Run(app_config *configuration.Config) {
	logger, _ := log.New("categorizer", log.WITH_TIMESTAMP)

	logger.Info("Starting by getting blockchain service parameters", "protocol", "inproc")

	blockchain_service, err := service.Inprocess(service.BLOCKCHAIN)
	if err != nil {
		logger.Fatal("failed to get inproc configuration for the service", "service type", service.BLOCKCHAIN, "error", err)
	}

	logger.Info("Create a blockchain client socket", "protocol", "url", blockchain_service.Url())

	blockchain_socket, err := remote.InprocRequestSocket(blockchain_service.Url(), logger, app_config)
	if err != nil {
		logger.Fatal("remote.InprocRequest", "url", blockchain_service.Url(), "error", err)
	}

	logger.Info("Get supported networks from blockchain", "network_type", network.ALL)

	request_parameters := blockchain_command.GetNetworksRequest{
		NetworkType: network.ALL,
	}

	var networks_parameters blockchain_command.GetNetworksReply
	err = blockchain_command.NETWORKS_COMMAND.Request(blockchain_socket, request_parameters, &networks_parameters)
	if err != nil {
		logger.Fatal("network.GetRemoteNetworks", "error", err)
	}
	if err := blockchain_socket.Close(); err != nil {
		logger.Fatal("blockchain client socket close", "error", err)
	}

	logger.Info("Networks returned from blockchain service, get network client sockets")

	network_sockets, err := network.NewClientSockets(app_config, logger)
	if err != nil {
		logger.Fatal("network.NewClientSockets", "error", err)
	}

	logger.Info("Get the database service parameters", "protocol", "inproc")

	database_service, err := service.Inprocess(service.DATABASE)
	if err != nil {
		logger.Fatal("service.Inprocess(service.DATABASE)", "error", err)
	}

	logger.Info("Create a database client socket", "protocol", "inproc", "url", database_service.Url())

	db_socket, err := remote.InprocRequestSocket(database_service.Url(), logger, app_config)
	if err != nil {
		logger.Fatal("remote.InprocRequestSocket", "error", err)
	}

	logger.Info("Creating new reply controller")

	cat_service := Service()
	reply, err := controller.NewReply(cat_service, logger)
	if err != nil {
		logger.Fatal("controller.NewReply", "service", Service())
	}

	err = reply.Run(CommandHandlers, db_socket, network_sockets, networks_parameters.Networks)
	if err != nil {
		logger.Fatal("controller.Run", "error", err)
	}
}
