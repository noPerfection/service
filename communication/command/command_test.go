package command

import (
	service "github.com/ahmetson/service-lib/identity"
	goLog "log"
	"testing"

	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/service-lib/communication/message"
	"github.com/ahmetson/service-lib/configuration"
	"github.com/ahmetson/service-lib/log"
	"github.com/ahmetson/service-lib/remote"
	zmq "github.com/pebbe/zmq4"
	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestCommandSuite struct {
	suite.Suite
	controller *zmq.Socket
	client     *remote.ClientSocket
}

// Make sure that Account is set to five
// before each test
func (suite *TestCommandSuite) SetupTest() {

	logger, err := log.New("command_test", true)
	suite.NoError(err, "failed to create logger")

	appConfig, err := configuration.New(logger)
	suite.NoError(err, "failed to create app config")

	shortUrl := "short"
	// at least len(protocol prefix) + 1 = 9 + 1
	_, err = remote.InprocRequestSocket(shortUrl, logger, appConfig)
	suite.Error(err)
	noProtocolUrl := "indexer"
	_, err = remote.InprocRequestSocket(noProtocolUrl, logger, appConfig)
	suite.Error(err)

	url := "inproc://test_proc"
	socket, err := remote.InprocRequestSocket(url, logger, appConfig)
	suite.NoError(err)

	controller, err := zmq.NewSocket(zmq.REP)
	suite.NoError(err)
	err = controller.Bind(url)
	suite.NoError(err)

	suite.controller = controller
	suite.client = socket
}

// All methods that begin with "Test" are run as tests within a
// suite.
func (suite *TestCommandSuite) TestRun() {
	go func() {
		// Test command.Request
		// Skip command.Push
		receiveMessage, err := suite.controller.RecvMessage(0)
		suite.NoError(err)
		request, err := message.ParseRequest(receiveMessage)
		suite.NoError(err)
		goLog.Println("received by controller", request, receiveMessage)

		reply := message.Reply{
			Status:     message.OK,
			Message:    "",
			Parameters: request.Parameters.Set("command", request.Command),
		}
		replyString, err := reply.String()
		suite.NoError(err)

		_, err = suite.controller.SendMessage(replyString)
		suite.NoError(err)

		// Test the router
		receiveMessage, err = suite.controller.RecvMessage(0)
		msgParts := make([]string, len(receiveMessage)-1)
		for i := 1; i < len(receiveMessage); i++ {
			msgParts[i-1] = receiveMessage[i]
		}

		suite.NoError(err)
		request, err = message.ParseRequest(msgParts)
		suite.NoError(err)

		reply = message.Reply{
			Status:  message.OK,
			Message: "",
			Parameters: request.Parameters.
				Set("command", request.Command).
				Set("router", receiveMessage[0]),
		}
		replyString, err = reply.String()
		suite.NoError(err)

		_, err = suite.controller.SendMessage(replyString)
		suite.NoError(err)

		_ = suite.controller.Close()
	}()

	// Test the Request
	command1 := Route{
		Command: "command_1",
	}
	requestParameters := key_value.Empty()
	var replyParameters key_value.KeyValue
	err := command1.Request(suite.client, requestParameters, &replyParameters)
	suite.NoError(err)
	suite.NotEmpty(replyParameters)
	replyCommandParam, err := replyParameters.GetString("command")
	suite.NoError(err)
	suite.Equal(command1.Command, replyCommandParam)

	// Test the Reply() function
	expectedReply := message.Reply{
		Status:     message.OK,
		Message:    "",
		Parameters: replyParameters,
	}
	createdReply, err := Reply(replyParameters)
	suite.NoError(err)
	suite.EqualValues(expectedReply, createdReply)

	// Test command.Push()
	url := "inproc://test_proc"
	client, err := zmq.NewSocket(zmq.PUSH)
	suite.NoError(err)
	err = client.Connect(url)
	suite.NoError(err)

	command2 := Route{
		Command: "command_2",
	}
	pushParameters := key_value.Empty()
	err = command2.Push(client, pushParameters)
	suite.NoError(err)

	indexerService, _ := service.Inprocess("INDEXER")

	// Test command.RequestRouter()
	command3 := Route{
		Command: "command_router",
	}
	err = command3.RequestRouter(suite.client, indexerService, requestParameters, &replyParameters)
	suite.NoError(err)
	repliedCommand, err := replyParameters.GetString("command")
	suite.NoError(err)
	suite.EqualValues(repliedCommand, command3.Command)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestCommand(t *testing.T) {
	suite.Run(t, new(TestCommandSuite))
}
