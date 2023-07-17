package controller

import (
	"sync"
	"testing"
	"time"

	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/service-lib/communication/command"
	"github.com/ahmetson/service-lib/communication/message"
	"github.com/ahmetson/service-lib/configuration"
	parameter "github.com/ahmetson/service-lib/identity"
	"github.com/ahmetson/service-lib/log"
	"github.com/ahmetson/service-lib/remote"
	zmq "github.com/pebbe/zmq4"
	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestRouterSuite struct {
	suite.Suite

	clientService *parameter.Service
	tcpRouter     *Router
	tcpRepliers   []*Controller
	tcpClient     *remote.ClientSocket
	logger        log.Logger

	commands []command.Name
}

// Todo test in-process and external types of controllers
// Todo test the business of the controller
// Make sure that Account is set to five
// before each test
func (suite *TestRouterSuite) SetupTest() {
	/////////////////////////////////////////////////////
	//
	// Services
	//
	/////////////////////////////////////////////////////

	// Logger and app configs are needed for External services
	logger, err := log.New("log", true)
	suite.NoError(err, "failed to create logger")
	appConfig, err := configuration.NewAppConfig(logger)
	suite.NoError(err, "failed to create logger")
	appConfig.SetDefault("SDS_REQUEST_TIMEOUT", 2)

	logger.Info("setup test")
	suite.logger = logger

	// Services
	clientService, err := parameter.NewExternal("CORE", parameter.REMOTE, appConfig)
	suite.Require().NoError(err)
	tcpService, err := parameter.NewExternal("CORE", parameter.THIS, appConfig)
	suite.Require().NoError(err, "failed to create indexer service")

	// Run the background Reply Controllers
	// Router's dealers will connect to them
	blockchainService, err := parameter.NewExternal("BLOCKCHAIN", parameter.THIS, appConfig)
	suite.Require().NoError(err, "failed to create blockchain service")

	////////////////////////////////////////////////////////
	//
	// Define the sockets
	//
	////////////////////////////////////////////////////////
	// client_service's limit is REMOTE, not this.
	// Router requires THIS limit
	_, err = NewRouter(clientService, logger)
	suite.Require().Error(err, "remote limited service should be failed as the parameter.Url() will not return wildcard host")
	tcpRouter, err := NewRouter(tcpService, logger)
	suite.Require().NoError(err)
	suite.tcpRouter = &tcpRouter

	// Client
	tcpClientSocket, err := remote.NewTcpSocket(clientService, &logger, appConfig)
	suite.Require().NoError(err, "failed to create subscriber socket")
	suite.tcpClient = tcpClientSocket

	// Reply Controllers
	blockchainSocket, err := NewReplier(logger)
	suite.Require().NoError(err, "remote limited service should be failed as the parameter.Url() will not return wildcard host")
	indexerSocket, err := NewReplier(logger)
	suite.Require().NoError(err, "remote limited service should be failed as the parameter.Url() will not return wildcard host")

	////////////////////////////////////////////////////
	//
	// Run the sockets
	//
	////////////////////////////////////////////////////
	command1 := command.New("command_1")
	command1Handler := func(request message.Request, _ log.Logger, _ remote.Clients) message.Reply {
		return message.Reply{
			Status:  message.OK,
			Message: "",
			Parameters: request.Parameters.
				Set("id", command1.String()).
				Set("dealer", "BLOCKCHAIN"),
		}
	}
	command2 := command.New("command_2")
	command2Handler := func(request message.Request, _ log.Logger, _ remote.Clients) message.Reply {
		logger.Info("reply back command", "service", "INDEXER")
		return message.Reply{
			Status:  message.OK,
			Message: "",
			Parameters: request.Parameters.
				Set("id", command2.String()).
				Set("dealer", "INDEXER"),
		}
	}
	blockchainSocket.RegisterCommand(command1, command1Handler)
	indexerSocket.RegisterCommand(command2, command2Handler)

	suite.commands = []command.Name{
		command1, command2,
	}

	// todo
	// add the reply controllers (BLOCKCHAIN, INDEXER)
	// assign to suite.<>_repliers
	//
	// Add to the router the BLOCKCHAIN, INDEXER, STORAGE
	//
	// send a command to in the goroutine -> loop
	// BUNDLE (should return error as not registered)
	// STORAGE (should return timeout from the client side)
	// BLOCKCHAIN
	// INDEXER

	suite.tcpRepliers = []*Controller{blockchainSocket, indexerSocket}
	go func() {
		_ = blockchainSocket.Run()
	}()
	go func() {
		_ = indexerSocket.Run()
	}()

	dealerBlockchain, err := parameter.NewExternal("BLOCKCHAIN", parameter.REMOTE, appConfig)
	suite.Require().NoError(err, "failed to create blockchain service")
	dealerIndexer, err := parameter.NewExternal("INDEXER", parameter.REMOTE, appConfig)
	suite.Require().NoError(err, "failed to create indexer service")
	// The STORAGE is registered on the router, but doesn't exist
	// On the backend side.
	dealerStorage, err := parameter.NewExternal("STORAGE", parameter.REMOTE, appConfig)
	suite.Require().NoError(err, "failed to create indexer service")

	err = suite.tcpRouter.AddDealers(blockchainService)
	suite.Require().Error(err, "failed to add dealer, because limit is THIS")
	err = suite.tcpRouter.AddDealers(dealerBlockchain, dealerIndexer, dealerStorage)
	suite.Require().NoError(err, "failed to create blockchain service")
	go suite.tcpRouter.Run()

	suite.clientService = clientService

	// Prepare for the controllers to be ready
	time.Sleep(time.Millisecond * 200)
}

// All methods that begin with "Test" are run as tests within a
// suite.
func (suite *TestRouterSuite) TestRun() {
	var wg sync.WaitGroup

	wg.Add(1)
	// tcp client
	go func() {
		for i := 0; i < 5; i++ {
			requestParameters := key_value.Empty().
				Set("counter", uint64(i))
			var replyParameters key_value.KeyValue

			commandIndex := 1
			dealer, _ := parameter.Inprocess("INDEXER")

			err := suite.commands[commandIndex].RequestRouter(suite.tcpClient, dealer, requestParameters, &replyParameters)
			suite.NoError(err)

			counter, err := replyParameters.GetUint64("counter")
			suite.Require().NoError(err)
			suite.Equal(counter, uint64(i))

			id, err := replyParameters.GetString("id")
			suite.Require().NoError(err)
			suite.Equal(id, suite.commands[commandIndex].String())
		}

		for i := 0; i < 5; i++ {
			requestParameters := key_value.Empty().
				Set("counter", uint64(i))
			var replyParameters key_value.KeyValue

			commandIndex := 0
			dealer, _ := parameter.Inprocess("BLOCKCHAIN")

			err := suite.commands[commandIndex].RequestRouter(suite.tcpClient, dealer, requestParameters, &replyParameters)
			suite.NoError(err)

			counter, err := replyParameters.GetUint64("counter")
			suite.Require().NoError(err)
			suite.Equal(counter, uint64(i))

			id, err := replyParameters.GetString("id")
			suite.Require().NoError(err)
			suite.Equal(id, suite.commands[commandIndex].String())
		}

		// no command found
		command3 := command.New("command_3")
		request3 := message.Request{
			Command:    command3.String(),
			Parameters: key_value.Empty(),
		}

		blockchainSocket, _ := parameter.Inprocess("BLOCKCHAIN")

		_, err := suite.tcpClient.RequestRouter(blockchainSocket, &request3)
		suite.Require().Error(err)

		suite.logger.Info("before requesting unhandled reply controller's dealer")

		storageSocket, _ := parameter.Inprocess("STORAGE")

		_, err = suite.tcpClient.RequestRouter(storageSocket, &request3)
		suite.Require().Error(err)

		suite.logger.Info("after requesting unhandled reply controller's dealer")

		wg.Done()
	}()
	wg.Wait()

	wg.Add(1)
	go func() {
		suite.logger.Info("test the high water mark, message over-buffer")
		socket, err := zmq.NewSocket(zmq.DEALER)
		if err != nil {
			suite.logger.Fatal("error creating socket: %w", err)
		}
		err = socket.Connect(suite.clientService.Url())
		if err != nil {
			suite.logger.Fatal("setup of dealer socket: %w", err)
		}

		for i := 1; i <= 2000; i++ {
			request := message.Request{
				Command: "no_existing",
			}
			requestString, _ := request.ToString()
			_, err = socket.SendMessage("STORAGE", requestString)
			suite.Require().NoError(err)
		}

		suite.logger.Info("Sent 2000 messages")
		wg.Done()
	}()

	wg.Wait()
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestRouter(t *testing.T) {
	suite.Run(t, new(TestRouterSuite))
}
