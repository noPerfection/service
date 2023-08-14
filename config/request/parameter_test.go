package request

import (
	"github.com/ahmetson/log-lib"
	"testing"
	"time"

	"github.com/ahmetson/service-lib/config"
	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing orchestra
type TestBroadcastSuite struct {
	suite.Suite
	appConfig *config.Config
}

// Make sure that Account is set to five
// before each test
func (suite *TestBroadcastSuite) SetupTest() {
	logger, err := log.New("request", true)
	suite.Require().NoError(err)

	appConfig, err := config.New(logger)
	suite.Require().NoError(err)

	suite.appConfig = appConfig
}

// All methods that begin with "Test" are run as tests within a
// suite.
func (suite *TestBroadcastSuite) TestDefaultValues() {
	suite.Require().Equal(DefaultTimeout, RequestTimeout(suite.appConfig))
	suite.Require().Equal(DefaultAttempt, Attempt(suite.appConfig))
}

func (suite *TestBroadcastSuite) TestZeroes() {
	suite.appConfig.SetDefault("SDS_REQUEST_TIMEOUT", uint64(0))
	suite.appConfig.SetDefault("SDS_REQUEST_ATTEMPT", uint64(0))

	suite.Require().Equal(DefaultTimeout, RequestTimeout(suite.appConfig))
	suite.Require().Equal(DefaultAttempt, Attempt(suite.appConfig))

	suite.appConfig.SetDefault("SDS_REQUEST_TIMEOUT", "not a number")
	suite.Require().Equal(DefaultTimeout, RequestTimeout(suite.appConfig))

	suite.appConfig.SetDefault("SDS_REQUEST_TIMEOUT", 74.65)
	suite.Require().Equal(time.Second*74, RequestTimeout(suite.appConfig))
}

func (suite *TestBroadcastSuite) TestValid() {
	suite.appConfig.SetDefault("SDS_REQUEST_TIMEOUT", uint64(5))
	suite.appConfig.SetDefault("SDS_REQUEST_ATTEMPT", uint64(10))

	suite.Require().Equal(time.Second*5, RequestTimeout(suite.appConfig))
	suite.Require().Equal(uint(10), Attempt(suite.appConfig))
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestParameter(t *testing.T) {
	suite.Run(t, new(TestBroadcastSuite))
}
