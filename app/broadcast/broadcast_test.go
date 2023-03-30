package broadcast

import (
	"testing"
	"time"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/app/service"
	"github.com/blocklords/sds/common/data_type/key_value"
	zmq "github.com/pebbe/zmq4"
	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestBroadcastSuite struct {
	suite.Suite
	subscriber *zmq.Socket
	broadcast  *Broadcast
	connection *zmq.Socket
}

// Make sure that Account is set to five
// before each test
func (suite *TestBroadcastSuite) SetupTest() {
	logger, err := log.New("log", log.WITHOUT_TIMESTAMP)
	suite.Require().Nil(err, "failed to create logger")

	app_config, err := configuration.NewAppConfig(logger)
	suite.Require().Nil(err, "failed to create logger")

	subscriber_service, err := service.NewExternal(service.CATEGORIZER, service.SUBSCRIBE, app_config)
	suite.Nil(err, "failed to create subscriber service")
	categorizer_service, err := service.NewExternal(service.CATEGORIZER, service.BROADCAST, app_config)
	suite.Nil(err, "failed to create categorizer service")

	// todo test the inproc broadcasting
	// todo add the exit
	_, err = New(subscriber_service, logger)
	suite.Error(err, "not the right limit")
	broadcast, err := New(categorizer_service, logger)
	suite.NoError(err, "failed to create broadcaster")

	connection_socket, err := ConnectionSocket(categorizer_service)
	suite.NoError(err, "failed to create connection socket")

	// Socket to talk to clients
	sub_socket, err := zmq.NewSocket(zmq.SUB)
	suite.NoError(err, "failed to create subscriber socket")
	err = sub_socket.Connect(subscriber_service.Url())
	suite.NoError(err, "failed to connect subscriber socket")

	suite.broadcast = broadcast
	suite.connection = connection_socket
	suite.subscriber = sub_socket

	go suite.broadcast.Run()
	// Prepare for the broadcaster to be ready.
	time.Sleep(time.Millisecond * 200)
}

// All methods that begin with "Test" are run as tests within a
// suite.
func (suite *TestBroadcastSuite) TestRun() {
	topic_title := "new_topic"
	go func() {
		for i := 0; i < 5; i++ {
			recv_message, err := suite.subscriber.RecvMessage(0)
			suite.NoError(err)
			recv_broadcast, err := message.ParseBroadcast(recv_message)
			suite.NoError(err)
			counter, _ := recv_broadcast.Reply.Parameters.GetUint64("counter")
			suite.Equal(counter, uint64(i))
			suite.Equal(topic_title, recv_broadcast.Topic)
		}
	}()

	for i := 0; i < 5; i++ {
		reply := message.Reply{
			Status:  message.OK,
			Message: "",
			Parameters: key_value.Empty().
				Set("counter", uint64(i)),
		}
		broadcast_message := message.NewBroadcast(topic_title, reply)

		broadcast_bytes := broadcast_message.ToBytes()
		_, err := suite.connection.SendBytes(broadcast_bytes, 0)
		suite.NoError(err)
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestBroadcast(t *testing.T) {
	suite.Run(t, new(TestBroadcastSuite))
}
