package config

import (
	"github.com/ahmetson/log-lib"
	"os"
	"testing"

	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing orchestra
type TestEnvSuite struct {
	suite.Suite
	envPath   string
	appConfig *Config
}

// Make sure that Account is set to five
// before each test
func (suite *TestEnvSuite) SetupTest() {
	os.Args = append(os.Args, "--plain")
	os.Args = append(os.Args, "--security-debug")
	os.Args = append(os.Args, "--number-key=5")

	envFile := "TRUE_KEY=true\n" +
		"FALSE_KEY=false\n" +
		"STRING_KEY=hello world\n" +
		"NUMBER_KEY=123\n" +
		"FLOAT_KEY=75.321\n"

	suite.envPath = ".test.env"
	os.Args = append(os.Args, suite.envPath)

	file, err := os.Create(suite.envPath)
	suite.Require().NoError(err)
	_, err = file.WriteString(envFile)
	suite.Require().NoError(err, "failed to write the data into: "+suite.envPath)
	err = file.Close()
	suite.Require().NoError(err, "delete the dump file: "+suite.envPath)

	logger, err := log.New("test_suite", true)
	suite.Require().NoError(err)
	appConfig, err := New(logger)
	suite.Require().NoError(err)
	suite.appConfig = appConfig

}

// All methods that begin with "Test" are run as tests within a
// suite.
func (suite *TestEnvSuite) TestRun() {
	suite.Require().False(suite.appConfig.Secure)
	suite.Require().NotNil(suite.appConfig.logger)

	suite.Require().False(suite.appConfig.Exist("TURKISH_KEY"))
	defaultConfig := DefaultConfig{
		Title: "TURKISH_KEYS",
		Parameters: key_value.Empty().
			// never will be written since env is already written
			Set("STRING_KEY", "salam").
			Set("TURKISH_KEY", "salam"),
	}
	suite.appConfig.SetDefaults(defaultConfig)
	suite.Require().True(suite.appConfig.Exist("TURKISH_KEY"))
	suite.Require().Equal(suite.appConfig.GetString("TURKISH_KEY"), "salam")

	key := "NOT_FOUND"
	value := "random_text"
	suite.Require().False(suite.appConfig.Exist(key))
	suite.Require().Empty(suite.appConfig.GetString(key))
	suite.appConfig.SetDefault(key, value)
	suite.Require().Equal(suite.appConfig.GetString(key), value)

	suite.Require().True(suite.appConfig.Exist("TRUE_KEY"))
	suite.Require().True(suite.appConfig.GetBool("TRUE_KEY"))
	suite.Require().True(suite.appConfig.Exist("FALSE_KEY"))
	suite.Require().False(suite.appConfig.GetBool("FALSE_KEY"))
	suite.Require().Equal(suite.appConfig.GetString("STRING_KEY"), "hello world")
	suite.Require().Equal(uint64(123), suite.appConfig.GetUint64("NUMBER_KEY"))
	suite.Require().True(suite.appConfig.Exist("FLOAT_KEY"))
	suite.Require().Equal(suite.appConfig.GetString("FLOAT_KEY"), "75.321")
	suite.Require().Empty(suite.appConfig.GetUint64("FLOAT_KEY"))

	err := os.Remove(suite.envPath)
	suite.Require().NoError(err, "delete the dump file: "+suite.envPath)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestCommand(t *testing.T) {
	suite.Run(t, new(TestEnvSuite))
}
