package inproc

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// We won't test the requests.
// The requests are tested in the controllers
// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestInprocSuite struct {
	suite.Suite
	network_id string
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
func (suite *TestInprocSuite) SetupTest() {
	suite.network_id = "1"
}

func (suite *TestInprocSuite) TestEndpoints() {
	client_endpoint := ClientEndpoint(suite.network_id)
	suite.Require().EqualValues("inproc://blockchain_1", client_endpoint)

	empty_endpoint := ClientEndpoint("")
	suite.Require().EqualValues("inproc://blockchain_", empty_endpoint)

	suite.Require().EqualValues("inproc://cat_recent_1", RecentIndexerEndpoint(suite.network_id))
	suite.Require().EqualValues("inproc://cat_recent_rep_1", RecentIndexerReplyEndpoint(suite.network_id))
	suite.Require().EqualValues("inproc://cat_old_1", OldIndexerEndpoint(suite.network_id))
	suite.Require().EqualValues("inproc://cat_1", IndexerEndpoint(suite.network_id))
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestInproc(t *testing.T) {
	suite.Run(t, new(TestInprocSuite))
}
