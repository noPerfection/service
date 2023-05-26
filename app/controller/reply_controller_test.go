package controller

import (
	"sync"
	"testing"
	"time"

	"github.com/blocklords/sds/app/communication/command"
	"github.com/blocklords/sds/app/communication/message"
	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/service"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestReplyControllerSuite struct {
	suite.Suite
	tcp_controller    *Controller
	inproc_controller *Controller
	tcp_client        *remote.ClientSocket
	inproc_client     *remote.ClientSocket
	commands          []command.CommandName
}

// Todo test inprocess and external types of controllers
// Todo test the business of the controller
// Make sure that Account is set to five
// before each test
func (suite *TestReplyControllerSuite) SetupTest() {
	logger, err := log.New("log", log.WITHOUT_TIMESTAMP)
	suite.NoError(err, "failed to create logger")
	app_config, err := configuration.NewAppConfig(logger)
	suite.NoError(err, "failed to create logger")

	client_service, err := service.NewExternal(service.CATEGORIZER, service.REMOTE, app_config)
	suite.Require().NoError(err)
	tcp_service, err := service.NewExternal(service.CATEGORIZER, service.THIS, app_config)
	suite.Require().NoError(err, "failed to create categorizer service")

	// todo test the inproc broadcasting
	// todo add the exit
	_, err = NewReply(client_service, logger)
	suite.Require().Error(err, "remote limited service should be failed as the service.Url() will not return wildcard host")
	tcp_controller, err := NewReply(tcp_service, logger)
	suite.NoError(err)
	suite.tcp_controller = tcp_controller

	inproc_service, err := service.Inprocess(service.CATEGORIZER)
	suite.NoError(err)
	suite.NotEmpty(inproc_service)

	inproc_controller, err := NewReply(inproc_service, logger)
	suite.NoError(err)
	suite.inproc_controller = inproc_controller

	// Socket to talk to clients
	tcp_client_socket, err := remote.NewTcpSocket(client_service, logger, app_config)
	suite.Require().NoError(err, "failed to create subscriber socket")
	suite.tcp_client = tcp_client_socket

	inproc_client_socket, err := remote.InprocRequestSocket(inproc_service.Url(), logger, app_config)
	suite.Require().NoError(err, "failed to connect subscriber socket")
	suite.inproc_client = inproc_client_socket

	command_1 := command.New("command_1")
	command_1_handler := func(request message.Request, _ log.Logger, _ ...interface{}) message.Reply {
		return message.Reply{
			Status:     message.OK,
			Message:    "",
			Parameters: request.Parameters.Set("id", command_1.String()),
		}
	}
	command_2 := command.New("command_2")
	command_2_handler := func(request message.Request, _ log.Logger, _ ...interface{}) message.Reply {
		return message.Reply{
			Status:     message.OK,
			Message:    "",
			Parameters: request.Parameters.Set("id", command_2.String()),
		}
	}
	handlers := command.EmptyHandlers().
		Add(command_1, command_1_handler).
		Add(command_2, command_2_handler)

	suite.commands = []command.CommandName{
		command_1, command_2,
	}

	go suite.inproc_controller.Run(handlers)
	go suite.tcp_controller.Run(handlers)

	// Prepare for the controllers to be ready
	time.Sleep(time.Millisecond * 200)
}

// All methods that begin with "Test" are run as tests within a
// suite.
func (suite *TestReplyControllerSuite) TestRun() {
	var wg sync.WaitGroup

	wg.Add(1)
	// tcp client
	go func() {
		for i := 0; i < 5; i++ {
			request_parameters := key_value.Empty().
				Set("counter", uint64(i))
			var reply_parameters key_value.KeyValue

			command_index := i % 2

			err := suite.commands[command_index].Request(suite.tcp_client, request_parameters, &reply_parameters)
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
		_, err := suite.tcp_client.RequestRemoteService(&request_3)
		suite.Require().Error(err)

		wg.Done()
	}()

	wg.Add(1)
	// tcp client
	go func() {
		for i := 0; i < 5; i++ {
			request_parameters := key_value.Empty().
				Set("counter", uint64(i))
			var reply_parameters key_value.KeyValue

			command_index := i % 2

			err := suite.commands[command_index].Request(suite.inproc_client, request_parameters, &reply_parameters)
			suite.NoError(err)

			counter, err := reply_parameters.GetUint64("counter")
			suite.Require().NoError(err)
			suite.Equal(counter, uint64(i))

			id, err := reply_parameters.GetString("id")
			suite.Require().NoError(err)
			suite.Equal(id, suite.commands[command_index].String())
		}
		wg.Done()
	}()

	wg.Wait()
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestReplyController(t *testing.T) {
	suite.Run(t, new(TestReplyControllerSuite))
}
