package remote

import (
	"testing"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/service"
	"github.com/stretchr/testify/suite"
)

// We won't test the requests.
// The requests are tested in the controllers
// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestSocketSuite struct {
	suite.Suite
}

// Test setup (inproc, tcp and sub)
//	Along with the reconnect
// Test Requests (router, remote)
// Test the timeouts
// Test close (attempt to request)

// Todo test inprocess and external types of controllers
// Todo test the business of the controller
// Make sure that Account is set to five
// before each test
func (suite *TestSocketSuite) SetupTest() {
}

func (suite *TestSocketSuite) TestNewSockets() {
	logger, err := log.New("log", log.WITHOUT_TIMESTAMP)
	suite.NoError(err, "failed to create logger")
	app_config, err := configuration.NewAppConfig(logger)
	suite.NoError(err, "failed to create logger")

	inproc_indexer_service, err := service.Inprocess(service.INDEXER)
	suite.Require().NoError(err)
	_, err = NewTcpSocket(inproc_indexer_service, logger, app_config)
	suite.Require().Error(err)

	indexer_service, err := service.NewExternal(service.INDEXER, service.THIS, app_config)
	suite.Require().NoError(err)
	client_service, err := service.NewExternal(service.INDEXER, service.REMOTE, app_config)
	suite.Require().NoError(err)
	subscriber_service, err := service.NewExternal(service.INDEXER, service.SUBSCRIBE, app_config)
	suite.Require().NoError(err)

	// We can't initiate the socket with the THIS limit
	_, err = NewTcpSocket(indexer_service, logger, app_config)
	suite.Require().Error(err)
	// We can't initiate with the empty service
	_, err = NewTcpSocket(client_service, logger, nil)
	suite.Require().Error(err)
	// We can't initiate with the empty service
	_, err = NewTcpSocket(nil, logger, app_config)
	suite.Require().Error(err)
	_, err = NewTcpSocket(client_service, logger, app_config)
	suite.Require().NoError(err)

	// We can't initiate the socket with the THIS limit
	_, err = InprocRequestSocket("", logger, app_config)
	suite.Require().Error(err)
	// We can't initiate with the empty service
	_, err = InprocRequestSocket("inproc://a", logger, nil)
	suite.Require().Error(err)
	// We can initiate with the empty service
	// but connnecting will fail during request
	_, err = InprocRequestSocket("inproc://", logger, app_config)
	suite.Require().NoError(err)
	// We can't initiate with the non inproc url
	_, err = InprocRequestSocket(indexer_service.Url(), logger, app_config)
	suite.Require().Error(err)
	_, err = InprocRequestSocket(inproc_indexer_service.Url(), logger, app_config)
	suite.Require().NoError(err)

	// We can't initiate the socket with the non SUBSCRIBE limit
	_, err = NewTcpSubscriber(indexer_service, "", nil, logger, app_config)
	suite.Require().Error(err)
	// We can't initiate with the empty service
	_, err = NewTcpSubscriber(subscriber_service, "", nil, logger, nil)
	suite.Require().Error(err)
	// We can't initiate with the empty service
	_, err = NewTcpSubscriber(nil, "", nil, logger, app_config)
	suite.Require().Error(err)
	_, err = NewTcpSubscriber(subscriber_service, "", nil, logger, app_config)
	suite.Require().NoError(err)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestReplyController(t *testing.T) {
	suite.Run(t, new(TestSocketSuite))
}
