package client

import (
	"testing"

	"github.com/ahmetson/log-lib"
	"github.com/ahmetson/service-lib/config"
	"github.com/stretchr/testify/suite"
)

// We won't test the requests.
// The requests are tested in the controllers
// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing orchestra
type TestSocketSuite struct {
	suite.Suite
}

// Test setup (inproc, tcp and sub)
//	Along with the reconnect
// Test Requests (router, client)
// Test the timeouts
// Test close (attempt to request)

// Todo test in-process and external types of controllers
// Todo test the business of the server
// Make sure that Account is set to five
// before each test
func (suite *TestSocketSuite) SetupTest() {
}

func (suite *TestSocketSuite) TestNewSockets() {
	logger, err := log.New("log", false)
	suite.NoError(err, "failed to create logger")
	appConfig, err := config.New(logger)
	suite.NoError(err, "failed to create logger")

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
