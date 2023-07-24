package argument

import (
	"fmt"
	"os"
	"testing"

	"github.com/ahmetson/service-lib/log"
	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestArgumentSuite struct {
	suite.Suite
	logger    *log.Logger
	arguments []string
}

// Make sure that Account is set to five
// before each test
func (suite *TestArgumentSuite) SetupTest() {
	os.Args = append(os.Args, "--plain")
	os.Args = append(os.Args, "--account")
	os.Args = append(os.Args, "--number-key=5")
	os.Args = append(os.Args, "./.test.env")

	logger, err := log.New("test_suite", false)
	suite.NoError(err)

	suite.arguments = []string{
		"plain",
		"account",
		"number-key=5",
	}
	suite.logger = logger
}

// All methods that begin with "Test" are run as tests within a
// suite.
func (suite *TestArgumentSuite) TestRun() {
	fmt.Println(os.Args)
	arguments := GetArguments()
	suite.Require().EqualValues(suite.arguments, arguments)

	pathArguments, _ := GetEnvPaths()
	suite.Require().Len(pathArguments, 1)
	pathArguments[0] = "./.test.env"

	// This -- prefixed key doesn't exist
	suite.False(Has(arguments, "not_exist"))
	// The .env variable doesn't exist
	suite.False(Has(arguments, "./.test.env"))
	// Key Value argument is returned
	suite.True(Has(arguments, "number-key"))
	suite.True(Has(arguments, "plain"))
	suite.True(Has(arguments, "account"))

	// Identical to argument.Has() except that
	// arguments are loaded from OS directly
	suite.False(Exist("not_exist"))
	// The .env variable doesn't exist
	suite.False(Exist("./.test.env"))
	// Key Value argument is returned
	suite.True(Exist("number-key"))
	suite.True(Exist("plain"))
	suite.True(Exist("account"))
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestCommand(t *testing.T) {
	suite.Run(t, new(TestArgumentSuite))
}
