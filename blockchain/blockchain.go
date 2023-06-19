// Package blockchain is the core service that acts as the gateway
// between other SDS services and blockchain network.
// All accesses to the blockchain network by SDS goes through blockchain service.
//
// Besides acting as the gateway, it also defines common blockchain data types:
//   - smartcontract events
//   - blockhain transaction
//
// Blockchain package also defines **network** sub package to handle the supported
// networks. Visit to [blockchain/network] for adding new supported networks.
//
// Each blockchain runs as a separate sds service.
//
// However their socket parameters are defined in [blockchain/inproc]
package blockchain

import (
	"github.com/blocklords/sds/service/configuration"
	"github.com/blocklords/sds/service/controller"
	parameter "github.com/blocklords/sds/service/identity"
	"github.com/blocklords/sds/service/log"

	"github.com/blocklords/sds/blockchain/handler"
	"github.com/blocklords/sds/blockchain/network"
)

////////////////////////////////////////////////////////////////////
//
// Command handlers
//
////////////////////////////////////////////////////////////////////

var CommandHandler = handler.CommandHandlers()

// Returns the parameter of the SDS Blockchain
func Service() *parameter.Service {
	service, _ := parameter.Inprocess(parameter.BLOCKCHAIN)
	return service
}

// Run the SDS Blockchain service.
// The SDS Blockchain will load the all supported networks from configuration.
//
// Then create the sub processes for each blockchain network.
//
// And finally enables the reply controller waiting for CommandHandlers
func Run(app_config *configuration.Config) {
	logger, _ := log.New("blockchain", log.WITH_TIMESTAMP)

	logger.Info("Setting default values for supported blockchain networks")
	app_config.SetDefault(network.SDS_BLOCKCHAIN_NETWORKS, network.DefaultConfiguration())

	this_service := Service()
	reply, err := controller.NewReply(this_service, logger)
	if err != nil {
		logger.Fatal("controller new", "message", err)
	}

	err = reply.Run(handler.CommandHandlers(), app_config)
	if err != nil {
		logger.Fatal("controller error", "error", err)
	}
}
