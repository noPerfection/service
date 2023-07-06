package env

import (
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestEnvSuite struct {
	suite.Suite
	envPath string
}

// Make sure that Account is set to five
// before each test
func (suite *TestEnvSuite) SetupTest() {
	suite.envPath = ".test.env"
	os.Args = append(os.Args, suite.envPath)

	file, err := os.Create(suite.envPath)
	suite.Require().NoError(err)
	_, err = file.WriteString("")
	suite.Require().NoError(err, "failed to write the data into: "+suite.envPath)
	err = file.Close()
	suite.Require().NoError(err, "delete the dump file: "+suite.envPath)
}

// All methods that begin with "Test" are run as tests within a
// suite.
func (suite *TestEnvSuite) TestRun() {
	err := LoadAnyEnv()
	suite.Require().NoError(err)
	err = os.Remove(suite.envPath)
	suite.Require().NoError(err, "delete the dump file: "+suite.envPath)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestCommand(t *testing.T) {
	suite.Run(t, new(TestEnvSuite))
}
