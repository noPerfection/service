package controller

import (
	"sync"
	"testing"
	"time"

	"github.com/blocklords/sds/app/command"
	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/app/service"
	"github.com/blocklords/sds/common/data_type/key_value"
	zmq "github.com/pebbe/zmq4"
	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestRouterSuite struct {
	suite.Suite

	client_service *service.Service
	tcp_router     *Router
	tcp_repliers   []*Controller
	tcp_client     *remote.ClientSocket
	logger         log.Logger

	commands []command.CommandName
}

// Todo test inprocess and external types of controllers
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
	logger, err := log.New("log", log.WITH_TIMESTAMP)
	suite.NoError(err, "failed to create logger")
	app_config, err := configuration.NewAppConfig(logger)
	suite.NoError(err, "failed to create logger")
	app_config.SetDefault("SDS_REQUEST_TIMEOUT", 2)

	logger.Info("setup test")
	suite.logger = logger

	// Services
	client_service, err := service.NewExternal(service.CORE, service.REMOTE, app_config)
	suite.Require().NoError(err)
	tcp_service, err := service.NewExternal(service.CORE, service.THIS, app_config)
	suite.Require().NoError(err, "failed to create categorizer service")

	// Run the background Reply Controllers
	// Router's dealers will connect to them
	blockchain_service, err := service.NewExternal(service.BLOCKCHAIN, service.THIS, app_config)
	suite.Require().NoError(err, "failed to create blockchain service")
	categorizer_service, err := service.NewExternal(service.CATEGORIZER, service.THIS, app_config)
	suite.Require().NoError(err, "failed to create categorizer service")

	////////////////////////////////////////////////////////
	//
	// Define the sockets
	//
	////////////////////////////////////////////////////////
	// client_service's limit is REMOTE, not this.
	// Router requires THIS limit
	_, err = NewRouter(client_service, logger)
	suite.Require().Error(err, "remote limited service should be failed as the service.Url() will not return wildcard host")
	tcp_router, err := NewRouter(tcp_service, logger)
	suite.Require().NoError(err)
	suite.tcp_router = &tcp_router

	// Client
	tcp_client_socket, err := remote.NewTcpSocket(client_service, logger, app_config)
	suite.Require().NoError(err, "failed to create subscriber socket")
	suite.tcp_client = tcp_client_socket

	// Reply Controllers
	blockchain_socket, err := NewReply(blockchain_service, logger)
	suite.Require().NoError(err, "remote limited service should be failed as the service.Url() will not return wildcard host")
	categorizer_socket, err := NewReply(categorizer_service, logger)
	suite.Require().NoError(err, "remote limited service should be failed as the service.Url() will not return wildcard host")

	////////////////////////////////////////////////////
	//
	// Run the sockets
	//
	////////////////////////////////////////////////////
	command_1 := command.New("command_1")
	command_1_handler := func(request message.Request, _ log.Logger, _ ...interface{}) message.Reply {
		return message.Reply{
			Status:  message.OK,
			Message: "",
			Parameters: request.Parameters.
				Set("id", command_1.String()).
				Set("dealer", service.BLOCKCHAIN.ToString()),
		}
	}
	command_2 := command.New("command_2")
	command_2_handler := func(request message.Request, _ log.Logger, _ ...interface{}) message.Reply {
		logger.Info("reply back command", "service", service.CATEGORIZER)
		return message.Reply{
			Status:  message.OK,
			Message: "",
			Parameters: request.Parameters.
				Set("id", command_2.String()).
				Set("dealer", service.CATEGORIZER.ToString()),
		}
	}
	blockchain_handlers := command.EmptyHandlers().
		Add(command_1, command_1_handler)

	categorizer_handlers := command.EmptyHandlers().
		Add(command_2, command_2_handler)

	suite.commands = []command.CommandName{
		command_1, command_2,
	}

	// todo
	// add the reply controllers (BLOCKCHAIN, CATEGORIZER)
	// assign to suite.<>_repliers
	//
	// Add to the router the BLOCKCHAIN, CATEGORIZER, STATIC
	//
	// send a command to in the goroutine -> loop
	// BUNDLE (should return error as not registered)
	// STATIC (should return timeout from the client side)
	// BLOCKCHAIN
	// CATEGORIZER

	suite.tcp_repliers = []*Controller{blockchain_socket, categorizer_socket}
	go blockchain_socket.Run(blockchain_handlers)
	go categorizer_socket.Run(categorizer_handlers)

	dealer_blockchain, err := service.NewExternal(service.BLOCKCHAIN, service.REMOTE, app_config)
	suite.Require().NoError(err, "failed to create blockchain service")
	dealer_categorizer, err := service.NewExternal(service.CATEGORIZER, service.REMOTE, app_config)
	suite.Require().NoError(err, "failed to create categorizer service")
	// The STATIC is registered on the router, but doesn't exist
	// On the backend side.
	dealer_static, err := service.NewExternal(service.STATIC, service.REMOTE, app_config)
	suite.Require().NoError(err, "failed to create categorizer service")

	err = suite.tcp_router.AddDealers(blockchain_service)
	suite.Require().Error(err, "failed to add dealer, because limit is THIS")
	err = suite.tcp_router.AddDealers(dealer_blockchain, dealer_categorizer, dealer_static)
	suite.Require().NoError(err, "failed to create blockchain service")
	go suite.tcp_router.Run()

	suite.client_service = client_service

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
			request_parameters := key_value.Empty().
				Set("counter", uint64(i))
			var reply_parameters key_value.KeyValue

			command_index := 1
			dealer, _ := service.Inprocess(service.CATEGORIZER)

			err := suite.commands[command_index].RequestRouter(suite.tcp_client, dealer, request_parameters, &reply_parameters)
			suite.NoError(err)

			counter, err := reply_parameters.GetUint64("counter")
			suite.Require().NoError(err)
			suite.Equal(counter, uint64(i))

			id, err := reply_parameters.GetString("id")
			suite.Require().NoError(err)
			suite.Equal(id, suite.commands[command_index].String())
		}

		for i := 0; i < 5; i++ {
			request_parameters := key_value.Empty().
				Set("counter", uint64(i))
			var reply_parameters key_value.KeyValue

			command_index := 0
			dealer, _ := service.Inprocess(service.BLOCKCHAIN)

			err := suite.commands[command_index].RequestRouter(suite.tcp_client, dealer, request_parameters, &reply_parameters)
			suite.NoError(err)

			counter, err := reply_parameters.GetUint64("counter")
			suite.Require().NoError(err)
			suite.Equal(counter, uint64(i))

			id, err := reply_parameters.GetString("id")
			suite.Require().NoError(err)
			suite.Equal(id, suite.commands[command_index].String())
		}

		// no command found
		command_3 := command.New("command_3")
		request_3 := message.Request{
			Command:    command_3.String(),
			Parameters: key_value.Empty(),
		}

		blockchain_socket, _ := service.Inprocess(service.BLOCKCHAIN)

		_, err := suite.tcp_client.RequestRouter(blockchain_socket, &request_3)
		suite.Require().Error(err)

		suite.logger.Info("before requesting unhandled reply controller's dealer")

		static_socket, _ := service.Inprocess(service.STATIC)

		_, err = suite.tcp_client.RequestRouter(static_socket, &request_3)
		suite.Require().Error(err)

		suite.logger.Info("after requesting unhandled reply controller's dealer")

		wg.Done()
	}()
	wg.Wait()

	wg.Add(1)
	go func() {
		suite.logger.Info("test the high water mark, message overbuffer")
		socket, err := zmq.NewSocket(zmq.DEALER)
		if err != nil {
			suite.logger.Fatal("error creating socket: %w", err)
		}
		err = socket.Connect(suite.client_service.Url())
		if err != nil {
			suite.logger.Fatal("setup of dealer socket: %w", err)
		}

		for i := 1; i <= 2000; i++ {
			request := message.Request{
				Command: "no_existing",
			}
			request_string, _ := request.ToString()
			_, err = socket.SendMessage(service.STATIC, request_string)
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
