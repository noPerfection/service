package auth

import (
	"testing"

	parameter "github.com/blocklords/sds/service/identity"
	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestServiceTypeSuite struct {
	suite.Suite
	service_type parameter.ServiceType
}

// Todo test inprocess and external types of controllers
// Todo test the business of the controller
// Make sure that Account is set to five
// before each test
func (suite *TestServiceTypeSuite) SetupTest() {
	suite.service_type = parameter.BUNDLE
}

func (suite *TestServiceTypeSuite) TestVaultPath() {
	name := vault_path(suite.service_type)
	suite.Equal("BUNDLE_SECRET_KEY", name)

	broadcast_name := vault_broadcast_path(suite.service_type)
	suite.Equal("BUNDLE_BROADCAST_SECRET_KEY", broadcast_name)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestServiceType(t *testing.T) {
	suite.Run(t, new(TestServiceTypeSuite))
}
