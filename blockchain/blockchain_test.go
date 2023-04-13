package blockchain

import (
	"testing"
	"time"

	"github.com/blocklords/sds/app/command"
	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/controller"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/app/service"
	"github.com/blocklords/sds/blockchain/handler"
	"github.com/blocklords/sds/blockchain/inproc"
	"github.com/blocklords/sds/blockchain/network"
	"github.com/blocklords/sds/blockchain/transaction"
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/smartcontract_key"
	"github.com/stretchr/testify/suite"
)

// We won't test the requests.
// The requests are tested in the controllers
// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestBlockchainSuite struct {
	suite.Suite
	clients key_value.KeyValue // clients to pass as app parameters to the command handlers

	evm_router *controller.Router
	imx_router *controller.Router
	commands   []command.CommandName

	app_config *configuration.Config
	logger     log.Logger
}

func (suite *TestBlockchainSuite) SetupTest() {
	// Logger and app configs are needed for External services
	logger, err := log.New("test-suite", log.WITH_TIMESTAMP)
	suite.NoError(err, "failed to create logger")
	app_config, err := configuration.NewAppConfig(logger)
	suite.NoError(err, "failed to create logger")
	app_config.SetDefault("SDS_REQUEST_TIMEOUT", 2)
	suite.app_config = app_config

	// set the default networks
	app_config.SetDefault(network.SDS_BLOCKCHAIN_NETWORKS, network.DefaultConfiguration())

	logger.Info("setup test")
	suite.logger = logger
}

func (suite *TestBlockchainSuite) TestDeployedTransaction() {
	// router services
	evm_router_service, err := service.NewExternal(service.EVM, service.THIS, suite.app_config)
	suite.Require().NoError(err, "failed to create categorizer service")
	imx_router_service, err := service.NewExternal(service.IMX, service.THIS, suite.app_config)
	suite.Require().NoError(err, "failed to create categorizer service")

	// Run the background Reply Controllers
	// Router's dealers will connect to them
	network_id_1 := "1"
	network_id_56 := "56"
	network_id_imx := "imx"
	network_1_categorizer_url := inproc.CategorizerEndpoint(network_id_1)
	network_56_categorizer_url := inproc.CategorizerEndpoint(network_id_56)
	network_imx_categorizer_url := inproc.CategorizerEndpoint(network_id_imx)

	network_1_client_url := inproc.ClientEndpoint(network_id_1)
	network_imx_client_url := inproc.ClientEndpoint(network_id_imx)
	network_56_client_url := inproc.ClientEndpoint(network_id_56)

	network_1_categorizer_service, _ := service.InprocessFromUrl(network_1_categorizer_url)
	network_56_categorizer_service, _ := service.InprocessFromUrl(network_56_categorizer_url)
	network_imx_categorizer_service, _ := service.InprocessFromUrl(network_imx_categorizer_url)

	network_1_client_service, _ := service.InprocessFromUrl(network_1_client_url)
	network_56_client_service, _ := service.InprocessFromUrl(network_56_client_url)
	network_imx_client_service, _ := service.InprocessFromUrl(network_imx_client_url)

	network_1_categorizer_reply, _ := controller.NewReply(network_1_categorizer_service, suite.logger)
	network_56_categorizer_reply, _ := controller.NewReply(network_56_categorizer_service, suite.logger)
	network_imx_categorizer_reply, _ := controller.NewReply(network_imx_categorizer_service, suite.logger)

	network_1_client_reply, _ := controller.NewReply(network_1_client_service, suite.logger)
	network_56_client_reply, _ := controller.NewReply(network_56_client_service, suite.logger)
	network_imx_client_reply, _ := controller.NewReply(network_imx_client_service, suite.logger)

	clients, _ := network.NewClientSockets(suite.app_config, suite.logger)
	suite.clients = clients

	////////////////////////////////////////////////////////
	//
	// Define the sockets
	//
	////////////////////////////////////////////////////////
	evm_router, err := controller.NewRouter(evm_router_service, suite.logger)
	suite.Require().NoError(err)
	suite.evm_router = &evm_router

	imx_router, err := controller.NewRouter(imx_router_service, suite.logger)
	suite.Require().NoError(err)
	suite.imx_router = &imx_router

	////////////////////////////////////////////////////
	//
	// The remote network service command handlers
	//
	////////////////////////////////////////////////////
	deployed_tx_command := handler.DEPLOYED_TRANSACTION_COMMAND
	command_1_handler := func(request message.Request, _ log.Logger, _ ...interface{}) message.Reply {
		raw_tx := transaction.RawTransaction{
			SmartcontractKey: smartcontract_key.Key{
				NetworkId: "sample network id",
				Address:   "sample contract",
			},
			BlockHeader: blockchain.BlockHeader{
				Number:    blockchain.Number(1),
				Timestamp: blockchain.Timestamp(2),
			},
			TransactionKey: blockchain.TransactionKey{
				Id:    "sample id",
				Index: 0,
			},
			From:  "unknown sender",
			Data:  "",
			Value: 0.0,
		}

		return message.Reply{
			Status:  message.OK,
			Message: "",
			Parameters: request.Parameters.
				Set("transaction", raw_tx),
		}
	}
	command_2 := command.New("command_2")
	command_2_handler := func(request message.Request, _ log.Logger, _ ...interface{}) message.Reply {
		suite.logger.Info("reply back command", "service", service.CATEGORIZER)
		return message.Reply{
			Status:  message.OK,
			Message: "",
			Parameters: request.Parameters.
				Set("id", command_2.String()).
				Set("dealer", service.CATEGORIZER.ToString()),
		}
	}
	client_handlers := command.EmptyHandlers().
		Add(deployed_tx_command, command_1_handler)

	categorizer_handlers := command.EmptyHandlers().
		Add(command_2, command_2_handler)

	suite.commands = []command.CommandName{
		deployed_tx_command, command_2,
	}

	///////////////////////////////////////////////////////////////
	// The network sub services
	// The categorization and client
	//
	// Categorization sends the command to the categorizer
	//
	// Client sends the command to the remote blockchain
	go network_1_categorizer_reply.Run(categorizer_handlers)
	go network_56_categorizer_reply.Run(categorizer_handlers)
	go network_imx_categorizer_reply.Run(categorizer_handlers)
	go network_1_client_reply.Run(client_handlers)
	go network_56_client_reply.Run(client_handlers)
	go network_imx_client_reply.Run(client_handlers)

	err = suite.evm_router.AddDealers(
		network_1_categorizer_service,
		network_56_categorizer_service,
		network_1_client_service,
		network_56_client_service,
	)
	suite.Require().NoError(err, "failed to add dealer, because limit is THIS")
	go suite.evm_router.Run()

	err = suite.imx_router.AddDealers(
		network_imx_categorizer_service,
		network_imx_client_service,
	)
	suite.Require().NoError(err, "failed to add dealer, because limit is THIS")
	go suite.imx_router.Run()

	// Prepare for the controllers to be ready
	time.Sleep(time.Millisecond * 200)

	///////////////////////////////////////////////
	//
	// Testing the deployed_transaction
	//
	///////////////////////////////////////////////

	request := message.Request{
		Command: "",
		Parameters: key_value.Empty().
			Set("network_id", "1").
			Set("transaction_id", "txid"),
	}
	// should fail as we don't pass app parameters
	reply := transaction_deployed_get(request, suite.logger)
	suite.Require().False(reply.IsOK())

	// should fail as we don't pass client sockets
	reply = transaction_deployed_get(request, suite.logger, suite.app_config)
	suite.Require().False(reply.IsOK())

	// should fail as the first app parameter should be app config
	reply = transaction_deployed_get(request, suite.logger, suite.clients, suite.app_config)
	suite.Require().False(reply.IsOK())

	// empty key value should fail
	request = message.Request{
		Command:    "",
		Parameters: key_value.Empty(),
	}
	reply = transaction_deployed_get(request, suite.logger, suite.app_config, suite.clients)
	suite.Require().False(reply.IsOK())

	// should fail as we don't have network id
	request.Parameters.Set("transaction_id", "2")
	reply = transaction_deployed_get(request, suite.logger, suite.app_config, suite.clients)
	suite.Require().False(reply.IsOK())

	// should fail as network id doesn't exist
	request.Parameters.Set("network_id", "unknown_network_id")
	reply = transaction_deployed_get(request, suite.logger, suite.app_config, suite.clients)
	suite.Require().False(reply.IsOK())

	// should fail as network clients are passed is nil
	request.Parameters.Set("network_id", "1")
	reply = transaction_deployed_get(request, suite.logger, suite.app_config)
	suite.Require().False(reply.IsOK())

	request.Parameters.Set("network_id", "1")
	reply = transaction_deployed_get(request, suite.logger, suite.app_config, suite.clients)
	suite.T().Log("the reply", reply)
	suite.Require().True(reply.IsOK())

	request.Parameters.Set("network_id", "56")
	reply = transaction_deployed_get(request, suite.logger, suite.app_config, suite.clients)
	suite.Require().True(reply.IsOK())
	suite.T().Log("the reply", reply.Parameters)

	request.Parameters.Set("network_id", "imx")
	reply = transaction_deployed_get(request, suite.logger, suite.app_config, suite.clients)
	suite.Require().True(reply.IsOK())
	suite.T().Log("the reply", reply.Parameters)
}

func (suite *TestBlockchainSuite) TestNetworks() {
	// check network
	request := message.Request{
		Command: "",
		Parameters: key_value.Empty().
			Set("network_id", "1").
			Set("network_type", "evm"),
	}
	// should be successful
	reply := get_network(request, suite.logger, suite.app_config, nil)
	suite.T().Log("replied network", reply)
	suite.Require().True(reply.IsOK())

	all_networks, _ := network.GetNetworks(suite.app_config, network.ALL)
	expected_network, _ := all_networks.Get("1")
	replied_network, _ := network.New(reply.Parameters)
	suite.Require().EqualValues(expected_network, replied_network)

	// fetching a network that is not on network type should fail
	//
	// network id "1" is not on imx
	request = message.Request{
		Command: "",
		Parameters: key_value.Empty().
			Set("network_id", "1").
			Set("network_type", "imx"),
	}
	reply = get_network(request, suite.logger, suite.app_config, nil)
	suite.Require().False(reply.IsOK())

	// fetching with empty parameter should fail
	request = message.Request{
		Command:    "",
		Parameters: key_value.Empty(),
	}
	reply = get_network(request, suite.logger, suite.app_config, nil)
	suite.Require().False(reply.IsOK())

	// fetching with missed data should fail
	request = message.Request{
		Command: "",
		Parameters: key_value.Empty().
			Set("network_id", "1"),
	}
	reply = get_network(request, suite.logger, suite.app_config, nil)
	suite.Require().False(reply.IsOK())

	// fetching with missed data should fail
	request = message.Request{
		Command: "",
		Parameters: key_value.Empty().
			Set("network_type", "evm"),
	}
	reply = get_network(request, suite.logger, suite.app_config, nil)
	suite.Require().False(reply.IsOK())

	// fetching network id with the wrong network type should fail
	request = message.Request{
		Command: "",
		Parameters: key_value.Empty().
			Set("network_id", "1").
			Set("network_type", "not_valid_network_type"),
	}
	reply = get_network(request, suite.logger, suite.app_config, nil)
	suite.Require().False(reply.IsOK())

	// fetching network id with the wrong network type should fail
	request = message.Request{
		Command: "",
		Parameters: key_value.Empty().
			Set("network_id", "imx").
			Set("network_type", "imx"),
	}
	reply = get_network(request, suite.logger, suite.app_config, nil)
	suite.Require().True(reply.IsOK())

}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestBlockchain(t *testing.T) {
	suite.Run(t, new(TestBlockchainSuite))
}
