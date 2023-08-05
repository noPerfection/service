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
	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestReplyControllerSuite struct {
	suite.Suite
	tcpController    *Controller
	inprocController *Controller
	tcpClient        *remote.ClientSocket
	inprocClient     *remote.ClientSocket
	commands         []command.Route
}

// Todo test in-process and external types of controllers
// Todo test the business of the controller
// Make sure that Account is set to five
// before each test
func (suite *TestReplyControllerSuite) SetupTest() {
	logger, err := log.New("log", false)
	suite.NoError(err, "failed to create logger")
	appConfig, err := configuration.New(logger)
	suite.NoError(err, "failed to create logger")

	clientService, err := parameter.NewExternal("INDEXER", parameter.REMOTE, appConfig)
	suite.Require().NoError(err)

	// todo test the inproc broadcasting
	// todo add the exit
	_, err = SyncReplier(logger)
	suite.Require().Error(err, "remote limited service should be failed as the parameter.Url() will not return wildcard host")
	tcpController, err := SyncReplier(logger)
	suite.NoError(err)
	suite.tcpController = tcpController

	inprocService, err := parameter.Inprocess("INDEXER")
	suite.NoError(err)
	suite.NotEmpty(inprocService)

	inprocController, err := SyncReplier(logger)
	suite.NoError(err)
	suite.inprocController = inprocController

	// Socket to talk to clients
	tcpClientSocket, err := remote.NewTcpSocket(clientService, logger, appConfig)
	suite.Require().NoError(err, "failed to create subscriber socket")
	suite.tcpClient = tcpClientSocket

	inprocClientSocket, err := remote.InprocRequestSocket(inprocService.Url(), logger, appConfig)
	suite.Require().NoError(err, "failed to connect subscriber socket")
	suite.inprocClient = inprocClientSocket

	command1 := command.Route{Command: "command_1"}
	var command1Handler = func(request message.Request, _ *log.Logger, _ ...*remote.ClientSocket) message.Reply {
		return message.Reply{
			Status:     message.OK,
			Message:    "",
			Parameters: request.Parameters.Set("id", command1.Command),
		}
	}
	_ = command1.AddHandler(command1Handler)

	command2 := command.Route{Command: "command_2"}
	command2Handler := func(request message.Request, _ *log.Logger, _ ...*remote.ClientSocket) message.Reply {
		return message.Reply{
			Status:     message.OK,
			Message:    "",
			Parameters: request.Parameters.Set("id", command2.Command),
		}
	}
	_ = command2.AddHandler(command2Handler)
	_ = suite.inprocController.AddRoute(&command1)
	_ = suite.inprocController.AddRoute(&command2)

	suite.commands = append(suite.commands, command1)
	suite.commands = append(suite.commands, command2)

	go func() {
		_ = suite.inprocController.Run()
	}()
	go func() {
		_ = suite.tcpController.Run()
	}()

	// Run for the controllers to be ready
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
			requestParameters := key_value.Empty().
				Set("counter", uint64(i))
			var replyParameters key_value.KeyValue

			commandIndex := i % 2

			err := suite.commands[commandIndex].Request(suite.tcpClient, requestParameters, &replyParameters)
			suite.NoError(err)

			counter, err := replyParameters.GetUint64("counter")
			suite.Require().NoError(err)
			suite.Equal(counter, uint64(i))

			id, err := replyParameters.GetString("id")
			suite.Require().NoError(err)
			suite.Equal(id, suite.commands[commandIndex].Command)
		}

		// no command found
		command3 := command.Route{Command: "command_3"}
		request3 := message.Request{
			Command:    command3.Command,
			Parameters: key_value.Empty(),
		}
		_, err := suite.tcpClient.RequestRemoteService(&request3)
		suite.Require().Error(err)

		wg.Done()
	}()

	wg.Add(1)
	// tcp client
	go func() {
		for i := 0; i < 5; i++ {
			requestParameters := key_value.Empty().
				Set("counter", uint64(i))
			var replyParameters key_value.KeyValue

			commandIndex := i % 2

			err := suite.commands[commandIndex].Request(suite.inprocClient, requestParameters, &replyParameters)
			suite.NoError(err)

			counter, err := replyParameters.GetUint64("counter")
			suite.Require().NoError(err)
			suite.Equal(counter, uint64(i))

			id, err := replyParameters.GetString("id")
			suite.Require().NoError(err)
			suite.Equal(id, suite.commands[commandIndex].Command)
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
