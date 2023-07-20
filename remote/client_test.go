package remote

import (
	"testing"

	"github.com/ahmetson/service-lib/configuration"
	parameter "github.com/ahmetson/service-lib/identity"
	"github.com/ahmetson/service-lib/log"
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

// Todo test in-process and external types of controllers
// Todo test the business of the controller
// Make sure that Account is set to five
// before each test
func (suite *TestSocketSuite) SetupTest() {
}

func (suite *TestSocketSuite) TestNewSockets() {
	logger, err := log.New("log", false)
	suite.NoError(err, "failed to create logger")
	appConfig, err := configuration.New(logger)
	suite.NoError(err, "failed to create logger")

	inprocIndexerService, err := parameter.Inprocess("indexer")
	suite.Require().NoError(err)
	_, err = NewTcpSocket(inprocIndexerService, &logger, appConfig)
	suite.Require().Error(err)

	indexerService, err := parameter.NewExternal("indexer", parameter.THIS, appConfig)
	suite.Require().NoError(err)
	clientService, err := parameter.NewExternal("indexer", parameter.REMOTE, appConfig)
	suite.Require().NoError(err)
	_, err = parameter.NewExternal("indexer", parameter.SUBSCRIBE, appConfig)
	suite.Require().NoError(err)

	// We can't initiate the socket with THIS limit
	_, err = NewTcpSocket(indexerService, &logger, appConfig)
	suite.Require().Error(err)
	// We can't initiate with the empty service
	_, err = NewTcpSocket(clientService, &logger, nil)
	suite.Require().Error(err)
	// We can't initiate with the empty service
	_, err = NewTcpSocket(nil, &logger, appConfig)
	suite.Require().Error(err)
	_, err = NewTcpSocket(clientService, &logger, appConfig)
	suite.Require().NoError(err)

	// We can't initiate the socket with THIS limit
	_, err = InprocRequestSocket("", logger, appConfig)
	suite.Require().Error(err)
	// We can't initiate with the empty service
	_, err = InprocRequestSocket("inproc://a", logger, nil)
	suite.Require().Error(err)
	// We can initiate with the empty service
	// but connecting will fail during request
	_, err = InprocRequestSocket("inproc://", logger, appConfig)
	suite.Require().NoError(err)
	// We can't initiate with the non inproc url
	_, err = InprocRequestSocket(indexerService.Url(), logger, appConfig)
	suite.Require().Error(err)
	_, err = InprocRequestSocket(inprocIndexerService.Url(), logger, appConfig)
	suite.Require().NoError(err)
	//
	//// We can't initiate the socket with the non SUBSCRIBE limit
	//_, err = NewTcpSubscriber(indexer_service, "", nil, logger, app_config)
	//suite.Require().Error(err)
	//// We can't initiate with the empty service
	//_, err = NewTcpSubscriber(subscriber_service, "", nil, logger, nil)
	//suite.Require().Error(err)
	//// We can't initiate with the empty service
	//_, err = NewTcpSubscriber(nil, "", nil, logger, app_config)
	//suite.Require().Error(err)
	//_, err = NewTcpSubscriber(subscriber_service, "", nil, logger, app_config)
	//suite.Require().NoError(err)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestReplyController(t *testing.T) {
	suite.Run(t, new(TestSocketSuite))
}
