package command

import (
	go_log "log"
	"testing"

	"github.com/Seascape-Foundation/sds-service-lib/common/data_type/key_value"
	"github.com/Seascape-Foundation/sds-service-lib/service/communication/message"
	"github.com/Seascape-Foundation/sds-service-lib/service/configuration"
	"github.com/Seascape-Foundation/sds-service-lib/service/log"
	"github.com/Seascape-Foundation/sds-service-lib/service/remote"
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

	logger, err := log.New("command_test", log.WITHOUT_TIMESTAMP)
	suite.NoError(err, "failed to create logger")

	app_config, err := configuration.NewAppConfig(logger)
	suite.NoError(err, "failed to create app config")

	short_url := "short"
	// atleast len(protocol prefix) + 1 = 9 + 1
	_, err = remote.InprocRequestSocket(short_url, logger, app_config)
	suite.Error(err)
	no_protocol_url := "indexer"
	_, err = remote.InprocRequestSocket(no_protocol_url, logger, app_config)
	suite.Error(err)

	url := "inproc://test_proc"
	socket, err := remote.InprocRequestSocket(url, logger, app_config)
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
		recv_message, err := suite.controller.RecvMessage(0)
		suite.NoError(err)
		request, err := message.ParseRequest(recv_message)
		suite.NoError(err)
		go_log.Println("received by controller", request, recv_message)

		reply := message.Reply{
			Status:     message.OK,
			Message:    "",
			Parameters: request.Parameters.Set("command", request.Command),
		}
		reply_string, err := reply.ToString()
		suite.NoError(err)

		_, err = suite.controller.SendMessage(reply_string)
		suite.NoError(err)

		// Test the router
		recv_message, err = suite.controller.RecvMessage(0)
		msg_parts := make([]string, len(recv_message)-1)
		for i := 1; i < len(recv_message); i++ {
			msg_parts[i-1] = recv_message[i]
		}

		suite.NoError(err)
		request, err = message.ParseRequest(msg_parts)
		suite.NoError(err)

		reply = message.Reply{
			Status:  message.OK,
			Message: "",
			Parameters: request.Parameters.
				Set("command", request.Command).
				Set("router", recv_message[0]),
		}
		reply_string, err = reply.ToString()
		suite.NoError(err)

		_, err = suite.controller.SendMessage(reply_string)
		suite.NoError(err)

		suite.controller.Close()
	}()

	// Test the Request
	command_1 := New("command_1")
	request_parameters := key_value.Empty()
	var reply_parameters key_value.KeyValue
	err := command_1.Request(suite.client, request_parameters, &reply_parameters)
	suite.NoError(err)
	suite.NotEmpty(reply_parameters)
	reply_command_param, err := reply_parameters.GetString("command")
	suite.NoError(err)
	suite.Equal(command_1.String(), reply_command_param)

	// Test the Reply() function
	expected_reply := message.Reply{
		Status:     message.OK,
		Message:    "",
		Parameters: reply_parameters,
	}
	created_reply, err := Reply(reply_parameters)
	suite.NoError(err)
	suite.EqualValues(expected_reply, created_reply)

	// Test command.Push()
	url := "inproc://test_proc"
	client, err := zmq.NewSocket(zmq.PUSH)
	suite.NoError(err)
	err = client.Connect(url)
	suite.NoError(err)

	command_2 := New("command_2")
	push_parameters := key_value.Empty()
	err = command_2.Push(client, push_parameters)
	suite.NoError(err)

	indexer_service, _ := service.Inprocess(service.INDEXER)

	// Test command.RequestRouter()
	command_3 := New("command_router")
	err = command_3.RequestRouter(suite.client, indexer_service, request_parameters, &reply_parameters)
	suite.NoError(err)
	replied_command, err := reply_parameters.GetString("command")
	suite.NoError(err)
	suite.EqualValues(replied_command, command_3.String())
	replied_router, err := reply_parameters.GetString("router")
	suite.NoError(err)
	suite.EqualValues(replied_router, service.INDEXER.ToString())
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestCommand(t *testing.T) {
	suite.Run(t, new(TestCommandSuite))
}
