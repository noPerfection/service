package identity

import (
	"testing"
	"time"

	"github.com/Seascape-Foundation/sds-service-lib/service/configuration"
	"github.com/Seascape-Foundation/sds-service-lib/service/log"
	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestBroadcastSuite struct {
	suite.Suite
	app_config *configuration.Config
}

// Make sure that Account is set to five
// before each test
func (suite *TestBroadcastSuite) SetupTest() {
	logger, err := log.New("parameter", log.WITH_TIMESTAMP)
	suite.Require().NoError(err)

	app_config, err := configuration.NewAppConfig(logger)
	suite.Require().NoError(err)

	suite.app_config = app_config
}

// All methods that begin with "Test" are run as tests within a
// suite.
func (suite *TestBroadcastSuite) TestDefaultValues() {
	suite.Require().Equal(REQUEST_TIMEOUT, RequestTimeout(suite.app_config))
	suite.Require().Equal(ATTEMPT, Attempt(suite.app_config))
}

func (suite *TestBroadcastSuite) TestZeroes() {
	suite.app_config.SetDefault("SDS_REQUEST_TIMEOUT", uint64(0))
	suite.app_config.SetDefault("SDS_REQUEST_ATTEMPT", uint64(0))

	suite.Require().Equal(REQUEST_TIMEOUT, RequestTimeout(suite.app_config))
	suite.Require().Equal(ATTEMPT, Attempt(suite.app_config))

	suite.app_config.SetDefault("SDS_REQUEST_TIMEOUT", "not a number")
	suite.Require().Equal(REQUEST_TIMEOUT, RequestTimeout(suite.app_config))

	suite.app_config.SetDefault("SDS_REQUEST_TIMEOUT", 74.65)
	suite.Require().Equal(time.Second*74, RequestTimeout(suite.app_config))
}

func (suite *TestBroadcastSuite) TestValid() {
	suite.app_config.SetDefault("SDS_REQUEST_TIMEOUT", uint64(5))
	suite.app_config.SetDefault("SDS_REQUEST_ATTEMPT", uint64(10))

	suite.Require().Equal(time.Second*5, RequestTimeout(suite.app_config))
	suite.Require().Equal(uint(10), Attempt(suite.app_config))
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestParameter(t *testing.T) {
	suite.Run(t, new(TestBroadcastSuite))
}
