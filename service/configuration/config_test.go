package configuration

import (
	"os"
	"testing"

	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/service/log"
	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestEnvSuite struct {
	suite.Suite
	env_path   string
	app_config *Config
}

// Make sure that Account is set to five
// before each test
func (suite *TestEnvSuite) SetupTest() {
	os.Args = append(os.Args, "--plain")
	os.Args = append(os.Args, "--security-debug")
	os.Args = append(os.Args, "--number-key=5")

	env_file := "TRUE_KEY=true\n" +
		"FALSE_KEY=false\n" +
		"STRING_KEY=hello world\n" +
		"NUMBER_KEY=123\n" +
		"FLOAT_KEY=75.321\n"

	suite.env_path = ".test.env"
	os.Args = append(os.Args, suite.env_path)

	file, err := os.Create(suite.env_path)
	suite.Require().NoError(err)
	_, err = file.WriteString(env_file)
	suite.Require().NoError(err, "failed to write the data into: "+suite.env_path)
	err = file.Close()
	suite.Require().NoError(err, "delete the dump file: "+suite.env_path)

	logger, err := log.New("test_suite", log.WITH_TIMESTAMP)
	suite.Require().NoError(err)
	app_config, err := NewAppConfig(logger)
	suite.Require().NoError(err)
	suite.app_config = app_config

}

// All methods that begin with "Test" are run as tests within a
// suite.
func (suite *TestEnvSuite) TestRun() {
	suite.Require().False(suite.app_config.Secure)
	suite.Require().True(suite.app_config.DebugSecurity)
	suite.Require().NotNil(suite.app_config.logger)

	suite.Require().False(suite.app_config.Exist("TURKISH_KEY"))
	default_config := DefaultConfig{
		Title: "TURKISH_KEYS",
		Parameters: key_value.Empty().
			// never will be written since env is already written
			Set("STRING_KEY", "merhaba dunye").
			Set("TURKISH_KEY", "merhaba"),
	}
	suite.app_config.SetDefaults(default_config)
	suite.Require().True(suite.app_config.Exist("TURKISH_KEY"))
	suite.Require().Equal(suite.app_config.GetString("TURKISH_KEY"), "merhaba")

	key := "NOT_FOUND"
	value := "random_text"
	suite.Require().False(suite.app_config.Exist(key))
	suite.Require().Empty(suite.app_config.GetString(key))
	suite.app_config.SetDefault(key, value)
	suite.Require().Equal(suite.app_config.GetString(key), value)

	suite.Require().True(suite.app_config.Exist("TRUE_KEY"))
	suite.Require().True(suite.app_config.GetBool("TRUE_KEY"))
	suite.Require().True(suite.app_config.Exist("FALSE_KEY"))
	suite.Require().False(suite.app_config.GetBool("FALSE_KEY"))
	suite.Require().Equal(suite.app_config.GetString("STRING_KEY"), "hello world")
	suite.Require().Equal(uint64(123), suite.app_config.GetUint64("NUMBER_KEY"))
	suite.Require().True(suite.app_config.Exist("FLOAT_KEY"))
	suite.Require().Equal(suite.app_config.GetString("FLOAT_KEY"), "75.321")
	suite.Require().Empty(suite.app_config.GetUint64("FLOAT_KEY"))

	err := os.Remove(suite.env_path)
	suite.Require().NoError(err, "delete the dump file: "+suite.env_path)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestCommand(t *testing.T) {
	suite.Run(t, new(TestEnvSuite))
}
