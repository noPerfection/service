package service

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestServiceTypeSuite struct {
	suite.Suite
}

// Todo test inprocess and external types of controllers
// Todo test the business of the controller
// Make sure that Account is set to five
// before each test
func (suite *TestServiceTypeSuite) SetupTest() {
	service := BUNDLE

	suite.Equal("BUNDLE", service.ToString())
}

func (suite *TestServiceTypeSuite) TestTypes() {
	types := service_types()
	suite.Require().Len(types, 9)
	suite.Equal(CORE, types[0])
	suite.Equal(SPAGHETTI, types[1])
	suite.Equal(CATEGORIZER, types[2])
	suite.Equal(STATIC, types[3])
	suite.Equal(GATEWAY, types[4])
	suite.Equal(DEVELOPER_GATEWAY, types[5])
	suite.Equal(READER, types[6])
	suite.Equal(WRITER, types[7])
	suite.Equal(BUNDLE, types[8])
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestServiceType(t *testing.T) {
	suite.Run(t, new(TestServiceTypeSuite))
}
