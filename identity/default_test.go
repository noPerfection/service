package identity

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestDefaultSuite struct {
	suite.Suite
}

// Todo test inprocess and external types of controllers
// Todo test the business of the controller
// Make sure that Account is set to five
// before each test
func (suite *TestDefaultSuite) SetupTest() {
	types := service_types()
	suite.Require().Len(types, 11)
	configs := DefaultConfigurations()
	suite.Require().Len(configs, len(types))

}

func (suite *TestDefaultSuite) TestRandom() {
	configs := DefaultConfigurations()
	suite.Equal("CORE", configs[0].Title)
	port, err := configs[2].Parameters.GetString("INDEXER_PORT")
	suite.Require().NoError(err)
	suite.Equal("4020", port)

	broadcast_port, err := configs[5].Parameters.GetString("DEVELOPER_GATEWAY_BROADCAST_PORT")
	suite.Require().NoError(err)
	suite.Equal("4051", broadcast_port)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestDefault(t *testing.T) {
	suite.Run(t, new(TestDefaultSuite))
}
