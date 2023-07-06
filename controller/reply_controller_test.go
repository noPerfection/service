package controller

import (
	"sync"
	"testing"
	"time"

	"github.com/Seascape-Foundation/sds-common-lib/data_type/key_value"
	"github.com/Seascape-Foundation/sds-service-lib/communication/command"
	"github.com/Seascape-Foundation/sds-service-lib/communication/message"
	"github.com/Seascape-Foundation/sds-service-lib/configuration"
	parameter "github.com/Seascape-Foundation/sds-service-lib/identity"
	"github.com/Seascape-Foundation/sds-service-lib/log"
	"github.com/Seascape-Foundation/sds-service-lib/remote"
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
	commands         []command.Name
}

// Todo test in-process and external types of controllers
// Todo test the business of the controller
// Make sure that Account is set to five
// before each test
func (suite *TestReplyControllerSuite) SetupTest() {
	logger, err := log.New("log", false)
	suite.NoError(err, "failed to create logger")
	appConfig, err := configuration.NewAppConfig(logger)
	suite.NoError(err, "failed to create logger")

	clientService, err := parameter.NewExternal(parameter.INDEXER, parameter.REMOTE, appConfig)
	suite.Require().NoError(err)
	tcpService, err := parameter.NewExternal(parameter.INDEXER, parameter.THIS, appConfig)
	suite.Require().NoError(err, "failed to create indexer service")

	// todo test the inproc broadcasting
	// todo add the exit
	_, err = NewReply(clientService, logger)
	suite.Require().Error(err, "remote limited service should be failed as the parameter.Url() will not return wildcard host")
	tcpController, err := NewReply(tcpService, logger)
	suite.NoError(err)
	suite.tcpController = tcpController

	inprocService, err := parameter.Inprocess(parameter.INDEXER)
	suite.NoError(err)
	suite.NotEmpty(inprocService)

	inprocController, err := NewReply(inprocService, logger)
	suite.NoError(err)
	suite.inprocController = inprocController

	// Socket to talk to clients
	tcpClientSocket, err := remote.NewTcpSocket(clientService, &logger, appConfig)
	suite.Require().NoError(err, "failed to create subscriber socket")
	suite.tcpClient = tcpClientSocket

	inprocClientSocket, err := remote.InprocRequestSocket(inprocService.Url(), logger, appConfig)
	suite.Require().NoError(err, "failed to connect subscriber socket")
	suite.inprocClient = inprocClientSocket

	command1 := command.New("command_1")
	command1Handler := func(request message.Request, _ log.Logger, _ ...interface{}) message.Reply {
		return message.Reply{
			Status:     message.OK,
			Message:    "",
			Parameters: request.Parameters.Set("id", command1.String()),
		}
	}
	command2 := command.New("command_2")
	command2Handler := func(request message.Request, _ log.Logger, _ ...interface{}) message.Reply {
		return message.Reply{
			Status:     message.OK,
			Message:    "",
			Parameters: request.Parameters.Set("id", command2.String()),
		}
	}
	handlers := command.EmptyHandlers().
		Add(command1, command1Handler).
		Add(command2, command2Handler)

	suite.commands = []command.Name{
		command1, command2,
	}

	go func() {
		_ = suite.inprocController.Run(handlers)
	}()
	go func() {
		_ = suite.tcpController.Run(handlers)
	}()

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
			suite.Equal(id, suite.commands[commandIndex].String())
		}

		// no command found
		command3 := command.New("command_3")
		request3 := message.Request{
			Command:    command3.String(),
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
			suite.Equal(id, suite.commands[commandIndex].String())
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
