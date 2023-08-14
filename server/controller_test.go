package server

import (
	"sync"
	"testing"
	"time"

	client "github.com/ahmetson/client-lib"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/common-lib/message"
	"github.com/ahmetson/log-lib"
	"github.com/ahmetson/service-lib/communication/command"
	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing orchestra
type TestReplyControllerSuite struct {
	suite.Suite
	tcpController    *Controller
	inprocController *Controller
	tcpClient        *client.ClientSocket
	inprocClient     *client.ClientSocket
	commands         []command.Route
}

// Todo test in-process and external types of controllers
// Todo test the business of the server
// Make sure that Account is set to five
// before each test
func (suite *TestReplyControllerSuite) SetupTest() {
	logger, err := log.New("log", false)
	suite.NoError(err, "failed to create logger")

	// todo test the inproc broadcasting
	// todo add the exit
	_, err = SyncReplier(logger)
	suite.Require().Error(err, "client limited service should be failed as the request.Url() will not return wildcard host")
	tcpController, err := SyncReplier(logger)
	suite.NoError(err)
	suite.tcpController = tcpController

	inprocController, err := SyncReplier(logger)
	suite.NoError(err)
	suite.inprocController = inprocController

	// Socket to talk to clients

	command1 := command.Route{Command: "command_1"}
	var command1Handler = func(request message.Request, _ *log.Logger, _ ...*client.ClientSocket) message.Reply {
		return message.Reply{
			Status:     message.OK,
			Message:    "",
			Parameters: request.Parameters.Set("id", command1.Command),
		}
	}
	_ = command1.AddHandler(command1Handler)

	command2 := command.Route{Command: "command_2"}
	command2Handler := func(request message.Request, _ *log.Logger, _ ...*client.ClientSocket) message.Reply {
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

	wg.Wait()
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestReplyController(t *testing.T) {
	suite.Run(t, new(TestReplyControllerSuite))
}
