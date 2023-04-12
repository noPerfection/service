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
	"fmt"

	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/common/data_type/database"
	"github.com/blocklords/sds/common/data_type/key_value"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/controller"
	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/service"
	blockchain_command "github.com/blocklords/sds/blockchain/handler"
	categorizer_process "github.com/blocklords/sds/blockchain/inproc"
	"github.com/blocklords/sds/blockchain/network"
	"github.com/blocklords/sds/categorizer/handler"
	"github.com/blocklords/sds/categorizer/smartcontract"
)

// Sends the smartcontracts to the blockchain package.
//
// The blockchain package will have the categorizer for its each blockchain type.
// They will handle the decoding the event logs.
// After decoding, the blockchain/categorizer will push back to this categorizer's puller.
func setup_smartcontracts(logger log.Logger, database_client *remote.ClientSocket, network *network.Network, client_socket *remote.ClientSocket) error {
	logger.Info("get all categorizable smartcontracts from database", "network_id", network.Id)

	var crud database.Crud = &smartcontract.Smartcontract{}
	condition := key_value.Empty().Set("network_id", network.Id)
	var smartcontracts []smartcontract.Smartcontract

	err := crud.SelectAllByCondition(database_client, condition, &smartcontracts)
	if err != nil {
		return fmt.Errorf("smartcontract.GetAllByNetworkId: %w", err)
	}
	if len(smartcontracts) == 0 {
		return nil
	}

	logger.Info("all smartcontracts returned", "network_id", network.Id, "smartcontract amount", len(smartcontracts))
	logger.Info("send smartcontracts to blockchain/categorizer", "network_id", network.Id, "url", categorizer_process.CategorizerEndpoint(network.Id))

	url := categorizer_process.CategorizerEndpoint(network.Id)
	categorizer_service, err := service.InprocessFromUrl(url)
	if err != nil {
		return fmt.Errorf("service.InprocessFromUrl(url): %w", err)
	}

	request := blockchain_command.PushNewSmartcontracts{
		Smartcontracts: smartcontracts,
	}
	var reply key_value.KeyValue
	err = blockchain_command.NEW_CATEGORIZED_SMARTCONTRACTS.RequestRouter(client_socket, categorizer_service, request, &reply)
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
func Run(app_config *configuration.Config, database_client *remote.ClientSocket) {
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

	var request_parameters = network.ALL
	var networks blockchain_command.GetNetworksReply
	err = blockchain_command.NETWORKS_COMMAND.Request(blockchain_socket, request_parameters, &networks)
	blockchain_socket.Close()
	if err != nil {
		logger.Fatal("newwork.GetRemoteNetworks", "error", err)
	}

	logger.Info("networks retreived")

	network_sockets, err := network.NewClientSockets(app_config, logger)
	if err != nil {
		logger.Fatal("network.NewClientSockets", "error", err)
	}

	for _, new_network := range networks {
		client_socket, ok := network_sockets[new_network.Type.String()].(*remote.ClientSocket)
		if !ok {
			logger.Fatal("no client socket to network service", "network id", new_network.Id, "network type", new_network.Type)
		}

		err = setup_smartcontracts(logger, database_client, new_network, client_socket)
		if err != nil {
			logger.Fatal("setup_smartcontracts", "network_id", new_network.Id, "error", err)
		}
	}

	cat_service := Service()
	reply, err := controller.NewReply(cat_service, logger)
	if err != nil {
		logger.Fatal("controller.NewReply", "service", Service())
	}

	err = reply.Run(CommandHandlers, database_client, network_sockets, networks)
	if err != nil {
		logger.Fatal("controller.Run", "error", err)
	}
}
